package debt

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xuri/excelize/v2"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func newTestContext() *DebtReceiptContext {
	return &DebtReceiptContext{
		PersonName:              "Anna",
		UserName:                "Viktor",
		GeneratedTime:           time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC),
		Timezone:                time.UTC,
		CategoryNames:           map[int64]string{7: "Groceries"},
		UnnamedReceiptTitle:     "Receipt",
		UnnamedTransactionTitle: "Transaction",
	}
}

func readTestSheet(t *testing.T, content []byte) ([][]string, string) {
	file, err := excelize.OpenReader(bytes.NewReader(content))
	assert.Nil(t, err)

	sheetName := file.GetSheetName(0)
	rows, err := file.GetRows(sheetName)
	assert.Nil(t, err)

	assert.Nil(t, file.Close())

	return rows, sheetName
}

func findTestRow(rows [][]string, firstCellValue string) []string {
	for i := 0; i < len(rows); i++ {
		if len(rows[i]) > 0 && rows[i][0] == firstCellValue {
			return rows[i]
		}
	}

	return nil
}

func TestWriteDebtReceiptXlsx_WritesHeadingAndTotal(t *testing.T) {
	entries := []*models.DebtEntryInfoResponse{
		{Id: 1, Amount: 1250, Currency: "EUR", TransactionTime: 1755648000, Name: "Taxi"},
		{Id: 2, Amount: 750, Currency: "EUR", TransactionTime: 1755734400, Name: "Cinema ticket"},
	}

	content, err := WriteDebtReceiptXlsx(entries, newTestContext())
	assert.Nil(t, err)

	rows, sheetName := readTestSheet(t, content)

	assert.Equal(t, "Anna", sheetName)
	assert.Equal(t, "Receipt", rows[0][0])
	assert.Equal(t, []string{"For", "Anna"}, findTestRow(rows, "For"))
	assert.Equal(t, []string{"From", "Viktor"}, findTestRow(rows, "From"))

	assert.NotNil(t, findTestRow(rows, "Issued"))

	headerRow := findTestRowByDescription(rows, "Description")
	assert.Equal(t, []string{"Date", "Description", "Where", "Note", "Amount"}, headerRow)

	// the two things owed add up to what the receipt asks for, and nothing else does
	totalRow := rows[len(rows)-1]
	assert.Equal(t, "Total", totalRow[3])
	assert.Equal(t, "20.00 EUR", totalRow[4])
}

func TestWriteDebtReceiptXlsx_ReadsOldestFirst(t *testing.T) {
	entries := []*models.DebtEntryInfoResponse{
		{Id: 1, Amount: 100, Currency: "EUR", TransactionTime: 1755734400, Name: "Bought second"},
		{Id: 2, Amount: 100, Currency: "EUR", TransactionTime: 1755648000, Name: "Bought first"},
	}

	content, err := WriteDebtReceiptXlsx(entries, newTestContext())
	assert.Nil(t, err)

	rows, _ := readTestSheet(t, content)
	descriptions := make([]string, 0, 2)

	for i := 0; i < len(rows); i++ {
		if len(rows[i]) > 1 && (rows[i][1] == "Bought first" || rows[i][1] == "Bought second") {
			descriptions = append(descriptions, rows[i][1])
		}
	}

	assert.Equal(t, []string{"Bought first", "Bought second"}, descriptions)
}

func TestWriteDebtReceiptXlsx_KeepsOneShoppingTripTogether(t *testing.T) {
	entries := []*models.DebtEntryInfoResponse{
		{Id: 1, Amount: 300, Currency: "EUR", TransactionTime: 1755648000, Name: "Bread", ReceiptId: 9, MerchantName: "Rewe"},
		{Id: 2, Amount: 700, Currency: "EUR", TransactionTime: 1755648000, Name: "Cheese", ReceiptId: 9, MerchantName: "Rewe"},
		{Id: 3, Amount: 500, Currency: "EUR", TransactionTime: 1755734400, Name: "Coffee", ReceiptId: 8, MerchantName: "Aldi"},
	}

	content, err := WriteDebtReceiptXlsx(entries, newTestContext())
	assert.Nil(t, err)

	rows, _ := readTestSheet(t, content)

	// the trip two things were owed off is a line of its own, and what it came to is on that line
	tripRow := findTestRowByDescription(rows, "Rewe")
	assert.NotNil(t, tripRow)
	assert.Equal(t, "10.00 EUR", tripRow[4])

	// its positions are written beneath it without repeating the day and the shop
	breadRow := findTestRowByDescription(rows, "Bread")
	assert.Equal(t, "", breadRow[0])
	assert.Equal(t, "", breadRow[2])

	// a trip only one thing was owed off stays an ordinary line, which says its own shop
	coffeeRow := findTestRowByDescription(rows, "Coffee")
	assert.Equal(t, "Aldi", coffeeRow[2])
	assert.NotEqual(t, "", coffeeRow[0])

	assert.Nil(t, findTestRowByDescription(rows, "Aldi"))
}

func TestWriteDebtReceiptXlsx_TotalsEachCurrencyOnItsOwn(t *testing.T) {
	entries := []*models.DebtEntryInfoResponse{
		{Id: 1, Amount: 1000, Currency: "EUR", TransactionTime: 1755648000, Name: "Dinner"},
		{Id: 2, Amount: 2000, Currency: "USD", TransactionTime: 1755734400, Name: "Museum"},
	}

	content, err := WriteDebtReceiptXlsx(entries, newTestContext())
	assert.Nil(t, err)

	rows, _ := readTestSheet(t, content)
	totals := make([]string, 0, 2)

	for i := 0; i < len(rows); i++ {
		if len(rows[i]) > 4 && rows[i][3] == "Total" {
			totals = append(totals, rows[i][4])
		}
	}

	assert.Equal(t, []string{"10.00 EUR", "20.00 USD"}, totals)

	// each bill says which currency it counts, because two of them are on the sheet
	assert.NotNil(t, findTestRow(rows, "EUR"))
	assert.NotNil(t, findTestRow(rows, "USD"))
}

func TestWriteDebtReceiptXlsx_NamesWhatHasNoNameOfItsOwn(t *testing.T) {
	entries := []*models.DebtEntryInfoResponse{
		{Id: 1, Amount: 500, Currency: "EUR", TransactionTime: 1755648000, CategoryId: 7},
		{Id: 2, Amount: 500, Currency: "EUR", TransactionTime: 1755734400, Missing: true},
	}

	content, err := WriteDebtReceiptXlsx(entries, newTestContext())
	assert.Nil(t, err)

	rows, _ := readTestSheet(t, content)

	// a transaction owed whole is called by its category, and one the ledger has forgotten is
	// still owed and still has to be called something
	assert.NotNil(t, findTestRowByDescription(rows, "Groceries"))
	assert.NotNil(t, findTestRowByDescription(rows, "Transaction"))
}

func TestBuildSheetName(t *testing.T) {
	assert.Equal(t, "Anna", buildSheetName("Anna"))
	assert.Equal(t, "Anna B", buildSheetName("Anna [B]"))
	assert.Equal(t, "Receipt", buildSheetName("  "))
	assert.Equal(t, "Receipt", buildSheetName("///"))
	assert.Equal(t, 31, len([]rune(buildSheetName("AnnaAndEverybodyElseWhoWasThereThatEvening"))))
}

func TestToSpreadsheetDate_KeepsTheDayItWasBoughtOn(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	assert.Nil(t, err)

	// half past midnight in Berlin is still the previous day in UTC, and the receipt must say the
	// day the buyer was standing in the shop
	instant := time.Date(2026, 8, 20, 22, 30, 0, 0, time.UTC)
	day := toSpreadsheetDate(instant, berlin)

	assert.Equal(t, 2026, day.Year())
	assert.Equal(t, time.August, day.Month())
	assert.Equal(t, 21, day.Day())
	assert.Equal(t, 0, day.Hour())
}

func findTestRowByDescription(rows [][]string, description string) []string {
	for i := 0; i < len(rows); i++ {
		if len(rows[i]) > 1 && rows[i][1] == description {
			return rows[i]
		}
	}

	return nil
}
