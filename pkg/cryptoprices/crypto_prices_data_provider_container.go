package cryptoprices

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// CryptoPricesDataProviderContainer contains the current crypto portfolio data provider
type CryptoPricesDataProviderContainer struct {
	current    CryptoPricesDataProvider
	configured bool
}

// Initialize a crypto portfolio data provider container singleton instance
var (
	Container = &CryptoPricesDataProviderContainer{}
)

// InitializeCryptoPricesDataSource initializes the current crypto data source according to the config
//
// A missing api key or share token is not a startup failure: the crypto page still shows the
// portfolio snapshot that was last retrieved, and only a refresh reports that it is not configured.
func InitializeCryptoPricesDataSource(config *settings.Config) error {
	if config.CryptoPricesDataSource == settings.CoinStatsCryptoPricesDataSource {
		Container.current = newCoinStatsDataSource(config)
		Container.configured = config.CoinStatsApiKey != "" && config.CoinStatsShareToken != ""
		return nil
	}

	return errs.ErrInvalidCryptoPricesDataSource
}

// IsConfigured returns whether the current crypto data source can be requested
func (p *CryptoPricesDataProviderContainer) IsConfigured() bool {
	return p.current != nil && p.configured
}

// GetPortfolio returns the portfolio from the current crypto data source
func (p *CryptoPricesDataProviderContainer) GetPortfolio(c core.Context, currency string) ([]*models.CryptoPortfolioCoinInfo, error) {
	if p.current == nil {
		return nil, errs.ErrInvalidCryptoPricesDataSource
	}

	if !p.configured {
		return nil, errs.ErrCryptoDataSourceNotConfigured
	}

	return p.current.GetPortfolio(c, currency)
}
