package converter

import (
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// ImportWarningCollector collects the non-fatal problems detected while parsing imported data.
// It is carried by TransactionDataImporterOptions as a pointer, so that an importer can report
// a warning back to the caller without changing the TransactionDataImporter interface.
type ImportWarningCollector struct {
	warnings []*models.ImportTransactionWarningResponse
}

// NewImportWarningCollector returns a new import warning collector instance
func NewImportWarningCollector() *ImportWarningCollector {
	return &ImportWarningCollector{
		warnings: make([]*models.ImportTransactionWarningResponse, 0),
	}
}

// Add appends a warning to the collector
func (c *ImportWarningCollector) Add(warning *models.ImportTransactionWarningResponse) {
	if c == nil || warning == nil {
		return
	}

	c.warnings = append(c.warnings, warning)
}

// GetWarnings returns all collected warnings
func (c *ImportWarningCollector) GetWarnings() []*models.ImportTransactionWarningResponse {
	if c == nil || len(c.warnings) < 1 {
		return nil
	}

	return c.warnings
}
