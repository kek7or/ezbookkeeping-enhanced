package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// CryptoAssetsApi represents crypto asset api
type CryptoAssetsApi struct {
	ApiUsingConfig
	prices *services.CryptoPricesService
}

// Initialize a crypto asset api singleton instance
var (
	CryptoAssets = &CryptoAssetsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		prices: services.CryptoPrices,
	}
)

// CryptoPortfolioHandler returns the cached crypto portfolio with what each coin is worth
//
// It refreshes the snapshot only when it has expired or when there is none, so opening the page
// normally costs the data source nothing.
func (a *CryptoAssetsApi) CryptoPortfolioHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	coins, err := a.prices.GetPortfolioCoins(c)

	if err != nil {
		log.Errorf(c, "[crypto_assets.CryptoPortfolioHandler] failed to get cached crypto portfolio for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if a.prices.RefreshPortfolioIfNeeded(c, len(coins) < 1) {
		coins, err = a.prices.GetPortfolioCoins(c)

		if err != nil {
			log.Errorf(c, "[crypto_assets.CryptoPortfolioHandler] failed to get refreshed crypto portfolio for user \"uid:%d\", because %s", uid, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	return a.buildCryptoPortfolioResponse(c, uid, coins)
}

// CryptoPortfolioRefreshHandler refreshes the cached crypto portfolio on explicit request
func (a *CryptoAssetsApi) CryptoPortfolioRefreshHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	err := a.prices.RefreshPortfolioManually(c)

	if err != nil {
		log.Warnf(c, "[crypto_assets.CryptoPortfolioRefreshHandler] failed to refresh crypto portfolio for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[crypto_assets.CryptoPortfolioRefreshHandler] user \"uid:%d\" has refreshed the crypto portfolio successfully", uid)

	return a.CryptoPortfolioHandler(c)
}

func (a *CryptoAssetsApi) buildCryptoPortfolioResponse(c *core.WebContext, uid int64, coins []*models.CryptoPortfolioCoin) (any, *errs.Error) {
	config := a.CurrentConfig()
	coinResponses := make([]*models.CryptoPortfolioCoinResponse, 0, len(coins))
	values := make([]int64, 0, len(coins))
	priceChanges := make([]string, 0, len(coins))
	totalValue := int64(0)
	totalValueIncomplete := false

	for i := 0; i < len(coins); i++ {
		coin := coins[i]
		coinResponse := &models.CryptoPortfolioCoinResponse{
			CoinId:        coin.CoinId,
			Symbol:        coin.Symbol,
			Name:          coin.Name,
			IconUrl:       coin.IconUrl,
			Count:         coin.Count,
			Price:         coin.Price,
			PriceChange1d: coin.PriceChange1d,
		}

		value, err := services.CalculateCryptoHoldingValue(coin.Count, coin.Price)

		if err != nil {
			// the data source did not give a usable price or quantity for this coin, so it is
			// listed without a value and the total says it is incomplete, rather than the coin
			// quietly counting as nothing
			log.Warnf(c, "[crypto_assets.buildCryptoPortfolioResponse] failed to value crypto coin \"%s\" for user \"uid:%d\", because %s", coin.CoinId, uid, err.Error())
			totalValueIncomplete = true
			coinResponses = append(coinResponses, coinResponse)
			continue
		}

		coinResponse.Value = &value
		totalValue += value
		values = append(values, value)
		priceChanges = append(priceChanges, coin.PriceChange1d)
		coinResponses = append(coinResponses, coinResponse)
	}

	state, err := a.prices.GetDataSourceState(c)

	if err != nil {
		log.Errorf(c, "[crypto_assets.buildCryptoPortfolioResponse] failed to get crypto data source state for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return &models.CryptoPortfolioResponse{
		Coins:                    coinResponses,
		ValueCurrency:            config.CryptoPriceCurrency,
		TotalValue:               totalValue,
		TotalValueIncomplete:     totalValueIncomplete,
		TotalPriceChange1d:       services.CalculateWeightedPriceChange(values, priceChanges),
		PricesUpdateTime:         state.LastPriceRefreshTime,
		NextRefreshAvailableTime: state.LastAttemptTime + int64(config.CryptoMinRefreshInterval),
		RequestsRemainingToday:   services.CryptoRequestsRemainingToday(state, config.CryptoMaxRequestsPerDay),
		LastErrorMessage:         state.LastErrorMessage,
		DataSourceConfigured:     a.prices.IsDataSourceConfigured(),
	}, nil
}
