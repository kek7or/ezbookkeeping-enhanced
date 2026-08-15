package converter

import (
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// ImportReceiptCollector collects the individual lines a receipt image was read as, so that the
// caller can return them next to the transactions they were aggregated into. It is carried by
// TransactionDataImporterOptions as a pointer, the same way warnings are reported, so that an
// importer can hand back more than the TransactionDataImporter interface returns.
//
// One request carries one image, so at most one receipt is ever collected.
type ImportReceiptCollector struct {
	receipt *models.ImportReceiptResponse
}

// NewImportReceiptCollector returns a new import receipt collector instance
func NewImportReceiptCollector() *ImportReceiptCollector {
	return &ImportReceiptCollector{}
}

// Set stores the lines of the recognized receipt
func (c *ImportReceiptCollector) Set(lineItems []*models.ImportReceiptLineItemResponse) {
	if c == nil || len(lineItems) < 1 {
		return
	}

	c.receipt = &models.ImportReceiptResponse{
		LineItems: lineItems,
	}
}

// GetReceipt returns the collected receipt, or nil when the import was not a receipt image
func (c *ImportReceiptCollector) GetReceipt() *models.ImportReceiptResponse {
	if c == nil {
		return nil
	}

	return c.receipt
}
