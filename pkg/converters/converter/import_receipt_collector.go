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

	if c.receipt == nil {
		c.receipt = &models.ImportReceiptResponse{}
	}

	c.receipt.LineItems = lineItems
}

// SetMerchantName stores the shop the receipt was printed by
func (c *ImportReceiptCollector) SetMerchantName(merchantName string) {
	if c == nil || merchantName == "" {
		return
	}

	if c.receipt == nil {
		c.receipt = &models.ImportReceiptResponse{}
	}

	c.receipt.MerchantName = merchantName
}

// SetPrintedTotal stores the total the till printed, in minor units.
//
// It is recorded separately from the sum of the lines because the two are different claims: this is
// what the paper says was paid, and where they disagree that disagreement is the point.
func (c *ImportReceiptCollector) SetPrintedTotal(printedTotal int64) {
	if c == nil {
		return
	}

	if c.receipt == nil {
		c.receipt = &models.ImportReceiptResponse{}
	}

	c.receipt.PrintedTotal = printedTotal
	c.receipt.HasPrintedTotal = true
}

// GetReceipt returns the collected receipt, or nil when the import was not a receipt image.
//
// A receipt whose lines could not be read is not returned at all, whatever else was recognized on it:
// the lines are what the client edits, and a merchant name with nothing under it is not something the
// user can be shown a receipt for.
func (c *ImportReceiptCollector) GetReceipt() *models.ImportReceiptResponse {
	if c == nil || c.receipt == nil || len(c.receipt.LineItems) < 1 {
		return nil
	}

	return c.receipt
}
