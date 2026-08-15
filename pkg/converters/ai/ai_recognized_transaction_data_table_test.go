package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/converters/datatable"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

func getTestRecognizedTransactionTime(t *testing.T, recognizedTime string) string {
	timezone := time.UTC
	dataTable, err := createNewAIRecognizedTransactionDataTable([]*models.RecognizedTransactionResult{
		{Type: "expense", Time: recognizedTime, Amount: "1.49"},
	}, timezone)

	assert.Nil(t, err)

	iterator := dataTable.TransactionRowIterator()
	assert.True(t, iterator.HasNext())

	dataRow, err := iterator.Next(core.NewNullContext(), &models.User{Uid: 1})
	assert.Nil(t, err)

	return dataRow.GetData(datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TIME)
}

func TestRecognizedTransactionTime_UsableValuesAreKept(t *testing.T) {
	assert.Equal(t, "2026-07-28 20:12:00", getTestRecognizedTransactionTime(t, "2026-07-28 20:12:00"))
	assert.Equal(t, "2026-07-28 20:12:00", getTestRecognizedTransactionTime(t, "  2026-07-28 20:12:00  "))

	// a time without seconds and a bare date are completed rather than rejected
	assert.Equal(t, "2026-07-28 20:12:00", getTestRecognizedTransactionTime(t, "2026-07-28 20:12"))
	assert.Equal(t, "2026-07-28 00:00:00", getTestRecognizedTransactionTime(t, "2026-07-28"))

	// an ISO 8601 answer is a real reading of the receipt, so it is converted, not discarded
	assert.Equal(t, "2026-07-28 18:12:39", getTestRecognizedTransactionTime(t, "2026-07-28T18:12:39.000Z"))
}

func TestRecognizedTransactionTime_UnusableValuesFallBackToNow(t *testing.T) {
	expectedTime := utils.FormatUnixTimeToLongDateTime(time.Now().Unix(), time.UTC)

	// none of these may fail the import, they all become the current time instead
	for _, recognizedTime := range []string{"", "   ", "28.07.2026 20:12", "gestern", "0000-00-00 00:00:00"} {
		actualTime := getTestRecognizedTransactionTime(t, recognizedTime)

		assert.True(t, utils.IsValidLongDateTimeFormat(actualTime), "recognized time %q produced %q", recognizedTime, actualTime)
		assert.Equal(t, expectedTime, actualTime, "recognized time %q", recognizedTime)
	}
}
