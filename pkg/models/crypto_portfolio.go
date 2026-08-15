package models

// CryptoPortfolioCoin represents one coin of the portfolio, as the data source last reported it
//
// The whole table is one snapshot of the remote portfolio and is replaced on every refresh, so a
// coin that has been sold disappears rather than lingering at its last quantity.
//
// The count and the price are decimal strings rather than numbers: no fixed int64 scale covers a
// meme coin position, and a float64 would not return the digits the data source printed. Every
// calculation on them goes through math/big.Rat.
type CryptoPortfolioCoin struct {
	CoinId        string `xorm:"PK VARCHAR(64) NOT NULL"`
	Symbol        string `xorm:"VARCHAR(16) NOT NULL"`
	Name          string `xorm:"VARCHAR(64) NOT NULL"`
	IconUrl       string `xorm:"VARCHAR(255) NOT NULL"`
	Currency      string `xorm:"VARCHAR(3) NOT NULL"`
	Count         string `xorm:"VARCHAR(40) NOT NULL"`
	Price         string `xorm:"VARCHAR(40) NOT NULL"`
	PriceChange1d string `xorm:"VARCHAR(16) NOT NULL"`
	// MarketRank is not named Rank because RANK is a reserved word in MySQL 8
	MarketRank      int32 `xorm:"NOT NULL"`
	DisplayOrder    int32 `xorm:"NOT NULL"`
	UpdatedUnixTime int64
}

// CryptoDataSourceState represents everything that has to be remembered between portfolio refreshes
// to keep the request count within the limits the data source plan allows
//
// There is exactly one row, with Id 1.
type CryptoDataSourceState struct {
	Id int64 `xorm:"PK"`
	// LastPriceRefreshTime is the last refresh that succeeded, and is what the cache lifetime
	// is measured against, so that a failing data source is retried rather than backed off from
	LastPriceRefreshTime int64 `xorm:"NOT NULL"`
	// LastAttemptTime is the last refresh that was started, successful or not. It is written
	// before the request is made, so it is also what stops two concurrent readers both fetching.
	LastAttemptTime  int64  `xorm:"NOT NULL"`
	LastErrorMessage string `xorm:"VARCHAR(255) NOT NULL"`
	// RequestCountDate is the UTC day RequestCountToday counts, as YYYYMMDD
	RequestCountDate  int32 `xorm:"NOT NULL"`
	RequestCountToday int32 `xorm:"NOT NULL"`
}

// CryptoDataSourceStateId is the primary key of the single crypto data source state row
const CryptoDataSourceStateId = int64(1)

// CryptoPortfolioCoinInfo represents one coin of the portfolio as a crypto data source returned it
type CryptoPortfolioCoinInfo struct {
	CoinId        string
	Symbol        string
	Name          string
	IconUrl       string
	Count         string
	Price         string
	PriceChange1d string
	MarketRank    int32
}

// CryptoPortfolioCoinResponse represents a view-object of one coin of the portfolio
type CryptoPortfolioCoinResponse struct {
	CoinId        string `json:"coinId"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	IconUrl       string `json:"iconUrl,omitempty"`
	Count         string `json:"count"`
	Price         string `json:"price,omitempty"`
	PriceChange1d string `json:"priceChange1d,omitempty"`
	// Value is in minor units of the value currency, and is nil when the coin has no usable
	// price, so that a coin nobody priced is never displayed as being worth nothing
	Value *int64 `json:"value"`
}

// CryptoPortfolioResponse represents a view-object of the whole portfolio and the state of the
// snapshot it was read from
type CryptoPortfolioResponse struct {
	Coins                    []*CryptoPortfolioCoinResponse `json:"coins"`
	ValueCurrency            string                         `json:"valueCurrency"`
	TotalValue               int64                          `json:"totalValue"`
	TotalValueIncomplete     bool                           `json:"totalValueIncomplete,omitempty"`
	TotalPriceChange1d       string                         `json:"totalPriceChange1d,omitempty"`
	PricesUpdateTime         int64                          `json:"pricesUpdateTime"`
	NextRefreshAvailableTime int64                          `json:"nextRefreshAvailableTime"`
	RequestsRemainingToday   int32                          `json:"requestsRemainingToday"`
	LastErrorMessage         string                         `json:"lastErrorMessage,omitempty"`
	DataSourceConfigured     bool                           `json:"dataSourceConfigured"`
}
