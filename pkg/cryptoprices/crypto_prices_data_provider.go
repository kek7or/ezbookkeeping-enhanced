package cryptoprices

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// CryptoPricesDataProvider defines the structure of crypto portfolio data provider
type CryptoPricesDataProvider interface {
	// GetPortfolio returns every coin of the configured portfolio, with what it is worth in the
	// requested currency. A coin the data source did not price is returned with an empty price
	// rather than a zero one.
	GetPortfolio(c core.Context, currency string) ([]*models.CryptoPortfolioCoinInfo, error)
}
