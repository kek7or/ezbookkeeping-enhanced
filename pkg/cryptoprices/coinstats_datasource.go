package cryptoprices

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/httpclient"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

const coinStatsPortfolioCoinsUrl = "https://openapiv1.coinstats.app/portfolio/coins"
const coinStatsApiKeyHeaderName = "X-API-KEY"
const coinStatsShareTokenHeaderName = "sharetoken"
const coinStatsPasscodeHeaderName = "passcode"
const coinStatsPortfolioCoinsPageLimit = 100

// coinStatsPortfolioCoinsResponse represents the response of the CoinStats portfolio coins
// endpoint. Only the fields this application reads are declared, so the data source adding
// fields can never break a refresh.
type coinStatsPortfolioCoinsResponse struct {
	Result []*coinStatsPortfolioCoin `json:"result"`
	Meta   *coinStatsResponseMeta    `json:"meta"`
}

type coinStatsResponseMeta struct {
	Page        int32 `json:"page"`
	PageCount   int32 `json:"pageCount"`
	HasNextPage bool  `json:"hasNextPage"`
}

// coinStatsPortfolioCoin represents one holding of the portfolio
//
// Count and the prices are read as json.Number so that the digits the data source printed are the
// digits that get stored. Decoding a price into a float64 and formatting it again is how
// "43250.75" turns into "43250.750000000004".
type coinStatsPortfolioCoin struct {
	Count json.Number             `json:"count"`
	Coin  *coinStatsCoin          `json:"coin"`
	Price map[string]*json.Number `json:"price"`
}

type coinStatsCoin struct {
	Identifier     string      `json:"identifier"`
	Symbol         string      `json:"symbol"`
	Name           string      `json:"name"`
	Icon           string      `json:"icon"`
	Rank           int32       `json:"rank"`
	PriceChange24h json.Number `json:"priceChange24h"`
}

// CoinStatsDataSource defines the structure of the CoinStats crypto data source
type CoinStatsDataSource struct {
	CryptoPricesDataProvider
	httpClient  *http.Client
	apiKey      string
	shareToken  string
	passcode    string
	portfolioId string
}

// GetPortfolio returns every coin of the configured CoinStats portfolio, with what it is worth in
// the requested currency
func (s *CoinStatsDataSource) GetPortfolio(c core.Context, currency string) ([]*models.CryptoPortfolioCoinInfo, error) {
	if s.apiKey == "" || s.shareToken == "" {
		return nil, errs.ErrCryptoDataSourceNotConfigured
	}

	allCoins := make([]*models.CryptoPortfolioCoinInfo, 0, coinStatsPortfolioCoinsPageLimit)

	// a portfolio larger than one page is rare, but paging through it here keeps a big portfolio
	// from being silently truncated to its first hundred coins
	for page := int32(1); ; page++ {
		coins, hasNextPage, err := s.getPortfolioPage(c, currency, page)

		if err != nil {
			return nil, err
		}

		allCoins = append(allCoins, coins...)

		if !hasNextPage || page >= 10 {
			break
		}
	}

	return allCoins, nil
}

func (s *CoinStatsDataSource) getPortfolioPage(c core.Context, currency string, page int32) ([]*models.CryptoPortfolioCoinInfo, bool, error) {
	req, err := s.buildPortfolioRequest(page)

	if err != nil {
		log.Errorf(c, "[coinstats_datasource.getPortfolioPage] failed to build request, because %s", err.Error())
		return nil, false, errs.ErrFailedToRequestRemoteApi
	}

	req = req.WithContext(httpclient.CustomHttpResponseLog(c, func(data []byte) {
		log.Debugf(c, "[coinstats_datasource.getPortfolioPage] response is %s", data)
	}))

	resp, err := s.httpClient.Do(req)

	if err != nil {
		log.Errorf(c, "[coinstats_datasource.getPortfolioPage] failed to request portfolio, because %s", err.Error())
		return nil, false, errs.ErrFailedToRequestRemoteApi
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Errorf(c, "[coinstats_datasource.getPortfolioPage] failed to read portfolio response, because %s", err.Error())
		return nil, false, errs.ErrFailedToRequestRemoteApi
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		log.Errorf(c, "[coinstats_datasource.getPortfolioPage] portfolio is not accessible, because response code is %d", resp.StatusCode)
		return nil, false, errs.ErrCryptoPortfolioNotAccessible
	}

	if resp.StatusCode != 200 {
		log.Errorf(c, "[coinstats_datasource.getPortfolioPage] failed to get portfolio response, because response code is %d", resp.StatusCode)
		return nil, false, errs.ErrFailedToRequestRemoteApi
	}

	return parseCoinStatsPortfolioResponse(body, currency)
}

func (s *CoinStatsDataSource) buildPortfolioRequest(page int32) (*http.Request, error) {
	queryParams := url.Values{}
	queryParams.Set("page", formatInt32(page))
	queryParams.Set("limit", formatInt32(coinStatsPortfolioCoinsPageLimit))

	if s.portfolioId != "" {
		queryParams.Set("portfolioId", s.portfolioId)
	}

	req, err := http.NewRequest("GET", coinStatsPortfolioCoinsUrl+"?"+queryParams.Encode(), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set(coinStatsApiKeyHeaderName, s.apiKey)
	req.Header.Set(coinStatsShareTokenHeaderName, s.shareToken)
	req.Header.Set("Accept", "application/json")

	if s.passcode != "" {
		req.Header.Set(coinStatsPasscodeHeaderName, s.passcode)
	}

	return req, nil
}

// parseCoinStatsPortfolioResponse returns the coins of a CoinStats portfolio response, and whether
// there is another page after this one
//
// A coin without a usable count or price is skipped rather than stored at zero, because "the data
// source did not tell us" and "this is worth nothing" must not look the same downstream.
func parseCoinStatsPortfolioResponse(content []byte, currency string) ([]*models.CryptoPortfolioCoinInfo, bool, error) {
	portfolioResponse := &coinStatsPortfolioCoinsResponse{}
	err := json.Unmarshal(content, portfolioResponse)

	if err != nil {
		return nil, false, errs.ErrFailedToRequestRemoteApi
	}

	coins := make([]*models.CryptoPortfolioCoinInfo, 0, len(portfolioResponse.Result))

	for i := 0; i < len(portfolioResponse.Result); i++ {
		portfolioCoin := portfolioResponse.Result[i]

		if portfolioCoin == nil || portfolioCoin.Coin == nil || portfolioCoin.Coin.Identifier == "" {
			continue
		}

		if portfolioCoin.Count.String() == "" {
			continue
		}

		coins = append(coins, &models.CryptoPortfolioCoinInfo{
			CoinId:        portfolioCoin.Coin.Identifier,
			Symbol:        strings.ToUpper(portfolioCoin.Coin.Symbol),
			Name:          portfolioCoin.Coin.Name,
			IconUrl:       portfolioCoin.Coin.Icon,
			Count:         portfolioCoin.Count.String(),
			Price:         getCoinStatsPrice(portfolioCoin.Price, currency),
			PriceChange1d: portfolioCoin.Coin.PriceChange24h.String(),
			MarketRank:    portfolioCoin.Coin.Rank,
		})
	}

	hasNextPage := portfolioResponse.Meta != nil && portfolioResponse.Meta.HasNextPage

	return coins, hasNextPage, nil
}

// getCoinStatsPrice returns the price in the requested currency
//
// The prices arrive keyed by currency, and only the requested one is taken: falling back to
// whichever key happened to come first would quietly report a value in the wrong currency.
func getCoinStatsPrice(prices map[string]*json.Number, currency string) string {
	if prices == nil {
		return ""
	}

	if price, exists := prices[currency]; exists && price != nil {
		return price.String()
	}

	if price, exists := prices[strings.ToUpper(currency)]; exists && price != nil {
		return price.String()
	}

	return ""
}

func formatInt32(value int32) string {
	return strconv.FormatInt(int64(value), 10)
}

func newCoinStatsDataSource(config *settings.Config) *CoinStatsDataSource {
	return &CoinStatsDataSource{
		httpClient:  httpclient.NewHttpClient(config.CryptoRequestTimeout, config.CryptoProxy, config.CryptoSkipTLSVerify, core.GetOutgoingUserAgent(), config.EnableDebugLog),
		apiKey:      config.CoinStatsApiKey,
		shareToken:  config.CoinStatsShareToken,
		passcode:    config.CoinStatsPasscode,
		portfolioId: config.CoinStatsPortfolioId,
	}
}
