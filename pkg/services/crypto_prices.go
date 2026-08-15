package services

import (
	"sync"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/cryptoprices"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// cryptoRefreshDecision represents whether the crypto portfolio may be refreshed right now
type cryptoRefreshDecision int

// Crypto portfolio refresh decisions
const (
	cryptoRefreshNotNeeded     cryptoRefreshDecision = 0
	cryptoRefreshAllowed       cryptoRefreshDecision = 1
	cryptoRefreshTooFrequent   cryptoRefreshDecision = 2
	cryptoRefreshLimitExceeded cryptoRefreshDecision = 3
)

// cryptoPortfolioDataSource is what the service needs from a crypto data source, declared here so
// that the refresh policy can be exercised without reaching a real one
type cryptoPortfolioDataSource interface {
	IsConfigured() bool
	GetPortfolio(c core.Context, currency string) ([]*models.CryptoPortfolioCoinInfo, error)
}

// CryptoPricesService represents the cached crypto portfolio and the policy that refreshes it
type CryptoPricesService struct {
	ServiceUsingDB
	ServiceUsingConfig
	dataSource  cryptoPortfolioDataSource
	refreshLock sync.Mutex
}

// Initialize a crypto prices service singleton instance
var (
	CryptoPrices = &CryptoPricesService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingConfig: ServiceUsingConfig{
			container: settings.Container,
		},
		dataSource: cryptoprices.Container,
	}
)

// IsDataSourceConfigured returns whether the crypto data source can be requested at all
func (s *CryptoPricesService) IsDataSourceConfigured() bool {
	return s.dataSource != nil && s.dataSource.IsConfigured()
}

// GetPortfolioCoins returns the cached portfolio snapshot
//
// Coins cached in a currency other than the one currently configured are left out, so that
// changing the price currency cannot make old numbers reappear under a new label.
func (s *CryptoPricesService) GetPortfolioCoins(c core.Context) ([]*models.CryptoPortfolioCoin, error) {
	var coins []*models.CryptoPortfolioCoin
	err := s.UserDataDB(0).NewSession(c).Where("currency=?", s.CurrentConfig().CryptoPriceCurrency).OrderBy("display_order asc").Find(&coins)

	return coins, err
}

// GetDataSourceState returns the current request accounting, creating it on first use
func (s *CryptoPricesService) GetDataSourceState(c core.Context) (*models.CryptoDataSourceState, error) {
	state := &models.CryptoDataSourceState{}
	has, err := s.UserDataDB(0).NewSession(c).Where("id=?", models.CryptoDataSourceStateId).Get(state)

	if err != nil {
		return nil, err
	}

	if !has {
		return &models.CryptoDataSourceState{Id: models.CryptoDataSourceStateId}, nil
	}

	return state, nil
}

// RefreshPortfolioIfNeeded refreshes the cached portfolio when it has expired or when there is no
// snapshot at all, and does nothing otherwise
//
// It never returns an error: reading the crypto page must succeed on the cached snapshot even when
// the data source is unreachable, and the reason it failed is reported through the state row.
func (s *CryptoPricesService) RefreshPortfolioIfNeeded(c core.Context, noSnapshot bool) bool {
	if !s.IsDataSourceConfigured() {
		return false
	}

	refreshed, err := s.refreshPortfolio(c, false, noSnapshot)

	if err != nil && err != errs.ErrCryptoPriceRefreshTooFrequent && err != errs.ErrCryptoPriceRefreshLimitExceeded {
		log.Warnf(c, "[crypto_prices.RefreshPortfolioIfNeeded] failed to refresh crypto portfolio, because %s", err.Error())
	}

	return refreshed
}

// RefreshPortfolioManually refreshes the cached portfolio on explicit request, and reports why it
// could not when the interval or the daily limit forbids it
func (s *CryptoPricesService) RefreshPortfolioManually(c core.Context) error {
	if !s.IsDataSourceConfigured() {
		return errs.ErrCryptoDataSourceNotConfigured
	}

	_, err := s.refreshPortfolio(c, true, false)

	return err
}

func (s *CryptoPricesService) refreshPortfolio(c core.Context, manual bool, noSnapshot bool) (bool, error) {
	s.refreshLock.Lock()
	defer s.refreshLock.Unlock()

	now := time.Now().Unix()
	decision, err := s.beginRefresh(c, now, manual, noSnapshot)

	if err != nil {
		return false, err
	}

	switch decision {
	case cryptoRefreshNotNeeded:
		return false, nil
	case cryptoRefreshTooFrequent:
		return false, errs.ErrCryptoPriceRefreshTooFrequent
	case cryptoRefreshLimitExceeded:
		return false, errs.ErrCryptoPriceRefreshLimitExceeded
	}

	currency := s.CurrentConfig().CryptoPriceCurrency
	coins, requestErr := s.dataSource.GetPortfolio(c, currency)

	if requestErr != nil {
		log.Errorf(c, "[crypto_prices.refreshPortfolio] failed to request crypto portfolio, because %s", requestErr.Error())

		// the cached snapshot is deliberately left untouched, so that a data source that is down
		// costs the last known values nothing
		if err := s.completeFailedRefresh(c, requestErr); err != nil {
			log.Errorf(c, "[crypto_prices.refreshPortfolio] failed to record the failed refresh, because %s", err.Error())
		}

		return false, requestErr
	}

	if err := s.savePortfolioSnapshot(c, coins, currency, time.Now().Unix()); err != nil {
		return false, err
	}

	return true, nil
}

// beginRefresh decides whether a refresh may happen and, when it may, books the request against
// today's allowance before it is made
//
// Writing the attempt before the request is what stops two readers arriving at once from both
// spending a request: the second one finds an attempt that is seconds old and stands down.
func (s *CryptoPricesService) beginRefresh(c core.Context, now int64, manual bool, noSnapshot bool) (cryptoRefreshDecision, error) {
	config := s.CurrentConfig()
	today := cryptoUtcDate(now)
	decision := cryptoRefreshNotNeeded

	err := s.UserDataDB(0).DoTransaction(c, func(sess *xorm.Session) error {
		state := &models.CryptoDataSourceState{}
		has, err := sess.Where("id=?", models.CryptoDataSourceStateId).Get(state)

		if err != nil {
			return err
		}

		if !has {
			state = &models.CryptoDataSourceState{Id: models.CryptoDataSourceStateId}

			if _, err := sess.Insert(state); err != nil {
				return err
			}
		}

		decision = decideCryptoRefresh(state, now, today, manual, noSnapshot, config)

		if decision != cryptoRefreshAllowed {
			return nil
		}

		state.LastAttemptTime = now

		if state.RequestCountDate != today {
			state.RequestCountDate = today
			state.RequestCountToday = 0
		}

		state.RequestCountToday = state.RequestCountToday + 1

		_, err = sess.Cols("last_attempt_time", "request_count_date", "request_count_today").Where("id=?", models.CryptoDataSourceStateId).Update(state)

		return err
	})

	if err != nil {
		return cryptoRefreshNotNeeded, err
	}

	return decision, nil
}

// savePortfolioSnapshot replaces the cached portfolio with what the data source just returned
//
// The old rows are deleted rather than merged, so a coin that has been sold disappears from the
// page instead of lingering forever at the quantity it last had.
func (s *CryptoPricesService) savePortfolioSnapshot(c core.Context, coins []*models.CryptoPortfolioCoinInfo, currency string, now int64) error {
	return s.UserDataDB(0).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := sess.Where("1=1").Delete(&models.CryptoPortfolioCoin{}); err != nil {
			return err
		}

		for i := 0; i < len(coins); i++ {
			coin := coins[i]

			if _, err := sess.Insert(&models.CryptoPortfolioCoin{
				CoinId:          coin.CoinId,
				Symbol:          coin.Symbol,
				Name:            coin.Name,
				IconUrl:         coin.IconUrl,
				Currency:        currency,
				Count:           coin.Count,
				Price:           coin.Price,
				PriceChange1d:   coin.PriceChange1d,
				MarketRank:      coin.MarketRank,
				DisplayOrder:    int32(i),
				UpdatedUnixTime: now,
			}); err != nil {
				return err
			}
		}

		stateModel := &models.CryptoDataSourceState{
			LastPriceRefreshTime: now,
			LastErrorMessage:     "",
		}

		_, err := sess.Cols("last_price_refresh_time", "last_error_message").Where("id=?", models.CryptoDataSourceStateId).Update(stateModel)

		return err
	})
}

func (s *CryptoPricesService) completeFailedRefresh(c core.Context, requestErr error) error {
	errorMessage := requestErr.Error()

	if len(errorMessage) > 255 {
		errorMessage = errorMessage[:255]
	}

	return s.UserDataDB(0).DoTransaction(c, func(sess *xorm.Session) error {
		stateModel := &models.CryptoDataSourceState{
			LastErrorMessage: errorMessage,
		}

		_, err := sess.Cols("last_error_message").Where("id=?", models.CryptoDataSourceStateId).Update(stateModel)

		return err
	})
}

// decideCryptoRefresh returns whether the portfolio may be refreshed now
//
// An automatic refresh only happens when the snapshot has expired or when there is none at all,
// which is what makes the first visit show something. Both kinds of refresh then pass through the
// same interval and daily limit, so neither can spend the data source allowance faster than the
// configuration permits.
func decideCryptoRefresh(state *models.CryptoDataSourceState, now int64, today int32, manual bool, noSnapshot bool, config *settings.Config) cryptoRefreshDecision {
	if !manual && !noSnapshot && now-state.LastPriceRefreshTime < int64(config.CryptoPriceCacheLifetime) {
		return cryptoRefreshNotNeeded
	}

	if now-state.LastAttemptTime < int64(config.CryptoMinRefreshInterval) {
		return cryptoRefreshTooFrequent
	}

	requestCountToday := int32(0)

	if state.RequestCountDate == today {
		requestCountToday = state.RequestCountToday
	}

	if requestCountToday >= int32(config.CryptoMaxRequestsPerDay) {
		return cryptoRefreshLimitExceeded
	}

	return cryptoRefreshAllowed
}

// CryptoRequestsRemainingToday returns how many requests today's allowance still has left
func CryptoRequestsRemainingToday(state *models.CryptoDataSourceState, maxRequestsPerDay uint16) int32 {
	return cryptoRequestsRemainingTodayAt(state, cryptoUtcDate(time.Now().Unix()), maxRequestsPerDay)
}

func cryptoRequestsRemainingTodayAt(state *models.CryptoDataSourceState, today int32, maxRequestsPerDay uint16) int32 {
	requestCountToday := int32(0)

	if state.RequestCountDate == today {
		requestCountToday = state.RequestCountToday
	}

	remaining := int32(maxRequestsPerDay) - requestCountToday

	if remaining < 0 {
		return 0
	}

	return remaining
}

// cryptoUtcDate returns a unix time as a YYYYMMDD number in UTC, which is the day the request
// allowance is counted over
func cryptoUtcDate(unixTime int64) int32 {
	t := time.Unix(unixTime, 0).UTC()
	return int32(t.Year()*10000 + int(t.Month())*100 + t.Day())
}
