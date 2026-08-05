package cryptoprices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// a CoinStats portfolio coins response, with fields this application does not read left in place
const coinStatsPortfolioResponse = `{
  "meta": { "page": 1, "limit": 100, "itemCount": 3, "pageCount": 1, "hasPreviousPage": false, "hasNextPage": false },
  "result": [
    {
      "count": 44.4987,
      "coin": {
        "rank": 2,
        "identifier": "ethereum",
        "symbol": "eth",
        "name": "Ethereum",
        "icon": "https://static.coinstats.app/coins/1650455629727.png",
        "priceChange24h": -5.74,
        "priceChange1h": 0.1,
        "volume": 61315198931.44
      },
      "price": { "USD": 4343.56, "BTC": 0.03948, "ETH": 1 },
      "profitPercent": { "allTime": { "USD": 83720.75 } }
    },
    {
      "count": 1000000000,
      "coin": {
        "rank": 15,
        "identifier": "shiba-inu",
        "symbol": "SHIB",
        "name": "Shiba Inu",
        "icon": "https://static.coinstats.app/coins/shib.png",
        "priceChange24h": -3.5
      },
      "price": { "USD": 0.00002341, "BTC": 0.0000000002 }
    },
    {
      "count": 5,
      "coin": { "identifier": "unpriced-coin", "symbol": "NONE", "name": "Unpriced Coin" },
      "price": { "BTC": 0.5 }
    }
  ]
}`

func TestParseCoinStatsPortfolioResponse(t *testing.T) {
	actualCoins, hasNextPage, err := parseCoinStatsPortfolioResponse([]byte(coinStatsPortfolioResponse), "USD")

	assert.Nil(t, err)
	assert.False(t, hasNextPage)
	assert.Equal(t, 3, len(actualCoins))

	assert.Equal(t, "ethereum", actualCoins[0].CoinId)
	assert.Equal(t, "ETH", actualCoins[0].Symbol)
	assert.Equal(t, "Ethereum", actualCoins[0].Name)
	assert.Equal(t, "https://static.coinstats.app/coins/1650455629727.png", actualCoins[0].IconUrl)
	assert.Equal(t, int32(2), actualCoins[0].MarketRank)
	assert.Equal(t, "-5.74", actualCoins[0].PriceChange1d)

	// the count and the price keep the digits the data source printed, which reading them into a
	// float64 and formatting them again would not
	assert.Equal(t, "44.4987", actualCoins[0].Count)
	assert.Equal(t, "4343.56", actualCoins[0].Price)
	assert.Equal(t, "1000000000", actualCoins[1].Count)
	assert.Equal(t, "0.00002341", actualCoins[1].Price)
}

func TestParseCoinStatsPortfolioResponse_PriceOnlyInAnotherCurrency(t *testing.T) {
	actualCoins, _, err := parseCoinStatsPortfolioResponse([]byte(coinStatsPortfolioResponse), "USD")

	assert.Nil(t, err)

	// the coin is kept, because the quantity is real, but it gets no price rather than a price
	// taken from whichever currency happened to be present
	assert.Equal(t, "unpriced-coin", actualCoins[2].CoinId)
	assert.Equal(t, "5", actualCoins[2].Count)
	assert.Equal(t, "", actualCoins[2].Price)
}

func TestParseCoinStatsPortfolioResponse_AnotherPriceCurrency(t *testing.T) {
	actualCoins, _, err := parseCoinStatsPortfolioResponse([]byte(coinStatsPortfolioResponse), "BTC")

	assert.Nil(t, err)
	assert.Equal(t, "0.03948", actualCoins[0].Price)
	assert.Equal(t, "0.5", actualCoins[2].Price)
}

func TestParseCoinStatsPortfolioResponse_CoinWithoutACountIsSkipped(t *testing.T) {
	actualCoins, _, err := parseCoinStatsPortfolioResponse([]byte(`{"result":[{"coin":{"identifier":"a-coin","symbol":"AC","name":"A Coin"},"price":{"USD":1}}]}`), "USD")

	assert.Nil(t, err)
	assert.Equal(t, 0, len(actualCoins))
}

func TestParseCoinStatsPortfolioResponse_HasNextPage(t *testing.T) {
	_, hasNextPage, err := parseCoinStatsPortfolioResponse([]byte(`{"meta":{"page":1,"pageCount":2,"hasNextPage":true},"result":[]}`), "USD")

	assert.Nil(t, err)
	assert.True(t, hasNextPage)
}

func TestParseCoinStatsPortfolioResponse_EmptyPortfolio(t *testing.T) {
	actualCoins, _, err := parseCoinStatsPortfolioResponse([]byte(`{"meta":{"itemCount":0},"result":[]}`), "USD")

	assert.Nil(t, err)
	assert.Equal(t, 0, len(actualCoins))
}

func TestParseCoinStatsPortfolioResponse_InvalidResponse(t *testing.T) {
	_, _, err := parseCoinStatsPortfolioResponse([]byte(`{"message":"Rate limit exceeded"`), "USD")

	assert.NotNil(t, err)
}

func TestCoinStatsBuildPortfolioRequest(t *testing.T) {
	dataSource := &CoinStatsDataSource{apiKey: "test-api-key", shareToken: "test-share-token"}
	req, err := dataSource.buildPortfolioRequest(1)

	assert.Nil(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "test-api-key", req.Header.Get(coinStatsApiKeyHeaderName))
	assert.Equal(t, "test-share-token", req.Header.Get(coinStatsShareTokenHeaderName))
	assert.Equal(t, "", req.Header.Get(coinStatsPasscodeHeaderName))
	assert.Equal(t, "openapiv1.coinstats.app", req.URL.Host)
	assert.Equal(t, "/portfolio/coins", req.URL.Path)
	assert.Equal(t, "1", req.URL.Query().Get("page"))
	assert.Equal(t, "100", req.URL.Query().Get("limit"))
	assert.Equal(t, "", req.URL.Query().Get("portfolioId"))
}

func TestCoinStatsBuildPortfolioRequest_WithPasscodeAndPortfolioId(t *testing.T) {
	dataSource := &CoinStatsDataSource{apiKey: "test-api-key", shareToken: "test-share-token", passcode: "1234", portfolioId: "abc"}
	req, err := dataSource.buildPortfolioRequest(2)

	assert.Nil(t, err)
	assert.Equal(t, "1234", req.Header.Get(coinStatsPasscodeHeaderName))
	assert.Equal(t, "abc", req.URL.Query().Get("portfolioId"))
	assert.Equal(t, "2", req.URL.Query().Get("page"))
}
