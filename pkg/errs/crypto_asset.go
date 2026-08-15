package errs

import "net/http"

// Error codes related to crypto assets
var (
	ErrCryptoDataSourceNotConfigured   = NewNormalError(NormalSubcategoryCryptoAsset, 0, http.StatusBadRequest, "crypto data source is not configured")
	ErrCryptoPortfolioNotAccessible    = NewNormalError(NormalSubcategoryCryptoAsset, 1, http.StatusBadRequest, "crypto portfolio cannot be read with the configured share token")
	ErrCryptoPriceRefreshTooFrequent   = NewNormalError(NormalSubcategoryCryptoAsset, 2, http.StatusBadRequest, "crypto portfolio has been refreshed recently")
	ErrCryptoPriceRefreshLimitExceeded = NewNormalError(NormalSubcategoryCryptoAsset, 3, http.StatusBadRequest, "daily crypto portfolio refresh limit has been exceeded")
	ErrInvalidCryptoAmount             = NewNormalError(NormalSubcategoryCryptoAsset, 4, http.StatusBadRequest, "crypto amount is invalid")
)
