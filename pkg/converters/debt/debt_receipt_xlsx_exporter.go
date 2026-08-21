package debt

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// The receipt is a spreadsheet somebody else reads. It is not an export of the ledger and not a
// backup - it is the piece of paper handed to the person who owes the money, and everything about
// it is decided by that: the positions are named the way the shop's own receipt named them, the
// trips are kept together the way they were bought, and the bottom line is the number that person
// has to pay.
//
// Dates and amounts are written as dates and amounts rather than as text, so that the sheet is
// still arithmetic when it arrives - the total can be checked, a row can be sorted, and the numbers
// are shown in whatever form the reader's own spreadsheet shows numbers in. A receipt that cannot
// be checked is only a picture of a receipt.

const (
	receiptColumnDate        = "A"
	receiptColumnDescription = "B"
	receiptColumnWhere       = "C"
	receiptColumnNote        = "D"
	receiptColumnAmount      = "E"
)

const receiptDefaultSheetName = "Receipt"

// receiptShortDateNumberFormat is Excel's built-in short date, which every spreadsheet renders in
// its own reader's convention. The day a thing was bought is the same day in every country and only
// its spelling differs, so the spelling is left to the machine that shows it.
const receiptShortDateNumberFormat = 14

// DebtReceiptContext is what the receipt knows beyond the things owed themselves: who it is for,
// who it is from, and the words and the clock to write it by.
type DebtReceiptContext struct {
	// PersonName is who owes the money and who the receipt is addressed to
	PersonName string
	// UserName is who paid and who the receipt is from, and may be empty when the user has no
	// nickname to be named by
	UserName string
	// GeneratedTime is when the receipt was drawn up, which is what dates it
	GeneratedTime time.Time
	// Timezone is the user's own clock. A transaction is stamped at an instant, but a receipt says
	// which day something was bought on, and which day an instant falls on is a question only a
	// timezone can answer.
	Timezone *time.Location
	// CategoryNames names the categories of the transactions owed whole, so that a transaction with
	// no receipt position behind it is called what it is called everywhere else in the program
	CategoryNames map[int64]string
	// UnnamedReceiptTitle is what a shopping trip whose shop was never named is called
	UnnamedReceiptTitle string
	// UnnamedTransactionTitle is what a transaction that can no longer say what it was for is called
	UnnamedTransactionTitle string
}

// receiptSection is everything owed in one currency.
//
// Currencies are never added together, so a receipt for somebody who was bought things in two of
// them is two bills on one sheet, each with its own total, rather than one total that is true of
// neither currency.
type receiptSection struct {
	currency string
	rows     []*receiptRow
	total    int64
}

// receiptRow is one line of the bill: a single thing owed, or a shopping trip several things were
// owed off, which is written as a line of its own with its positions beneath it
type receiptRow struct {
	entry       *models.DebtEntryInfoResponse
	tripEntries []*models.DebtEntryInfoResponse
	tripName    string
	tripTotal   int64
}

// receiptStyles are the cell styles of one workbook, made once and used for every row
type receiptStyles struct {
	title      int
	label      int
	value      int
	valueDate  int
	header     int
	date       int
	tripName   int
	position   int
	note       int
	section    int
	totalLabel int
}

// receiptMoneyStyles are the three ways an amount is shown on a bill: a position, what a trip came
// to, and what is owed altogether
type receiptMoneyStyles struct {
	amount int
	trip   int
	total  int
}

// WriteDebtReceiptXlsx writes what a person still owes as a spreadsheet.
//
// The entries are expected to be what is still open - a receipt says what is to be paid, and a row
// on it that has already been paid back would only invite paying it twice.
func WriteDebtReceiptXlsx(entries []*models.DebtEntryInfoResponse, context *DebtReceiptContext) ([]byte, error) {
	file := excelize.NewFile()

	defer func() {
		_ = file.Close()
	}()

	sheetName := buildSheetName(context.PersonName)

	if err := file.SetSheetName(file.GetSheetName(0), sheetName); err != nil {
		return nil, err
	}

	styles, err := buildReceiptStyles(file)

	if err != nil {
		return nil, err
	}

	if err = setReceiptColumnWidths(file, sheetName); err != nil {
		return nil, err
	}

	sections := buildReceiptSections(entries)
	row := 1

	if row, err = writeReceiptHeading(file, sheetName, styles, context, row); err != nil {
		return nil, err
	}

	for i := 0; i < len(sections); i++ {
		if row, err = writeReceiptSection(file, sheetName, styles, context, sections[i], len(sections) > 1, row); err != nil {
			return nil, err
		}
	}

	buffer, err := file.WriteToBuffer()

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// writeReceiptHeading writes who the receipt is for, who it is from and when it was drawn up, and
// returns the row the bill itself starts on
func writeReceiptHeading(file *excelize.File, sheetName string, styles *receiptStyles, context *DebtReceiptContext, row int) (int, error) {
	if err := setCell(file, sheetName, receiptColumnDate, row, receiptDefaultSheetName, styles.title); err != nil {
		return row, err
	}

	row += 2

	headings := []struct {
		label string
		value string
	}{
		{label: "For", value: context.PersonName},
		{label: "From", value: context.UserName},
	}

	for i := 0; i < len(headings); i++ {
		if headings[i].value == "" {
			continue
		}

		if err := setCell(file, sheetName, receiptColumnDate, row, headings[i].label, styles.label); err != nil {
			return row, err
		}

		if err := setCell(file, sheetName, receiptColumnDescription, row, headings[i].value, styles.value); err != nil {
			return row, err
		}

		row++
	}

	if err := setCell(file, sheetName, receiptColumnDate, row, "Issued", styles.label); err != nil {
		return row, err
	}

	if err := setCell(file, sheetName, receiptColumnDescription, row, toSpreadsheetDate(context.GeneratedTime, context.Timezone), styles.valueDate); err != nil {
		return row, err
	}

	return row + 2, nil
}

// writeReceiptSection writes everything owed in one currency, and returns the row after it.
//
// The currency is only announced when there is more than one of them. A receipt in the single
// currency everything was bought in says so on every amount already, and a heading over it would be
// telling the reader something they can see.
func writeReceiptSection(file *excelize.File, sheetName string, styles *receiptStyles, context *DebtReceiptContext, section *receiptSection, announceCurrency bool, row int) (int, error) {
	money, err := buildMoneyStyles(file, section.currency)

	if err != nil {
		return row, err
	}

	if announceCurrency {
		if err = setCell(file, sheetName, receiptColumnDate, row, section.currency, styles.section); err != nil {
			return row, err
		}

		row++
	}

	columns := []struct {
		column string
		title  string
	}{
		{column: receiptColumnDate, title: "Date"},
		{column: receiptColumnDescription, title: "Description"},
		{column: receiptColumnWhere, title: "Where"},
		{column: receiptColumnNote, title: "Note"},
		{column: receiptColumnAmount, title: "Amount"},
	}

	for i := 0; i < len(columns); i++ {
		if err = setCell(file, sheetName, columns[i].column, row, columns[i].title, styles.header); err != nil {
			return row, err
		}
	}

	row++

	for i := 0; i < len(section.rows); i++ {
		if row, err = writeReceiptRow(file, sheetName, styles, context, section.rows[i], money, row); err != nil {
			return row, err
		}
	}

	if err = setCell(file, sheetName, receiptColumnNote, row, "Total", styles.totalLabel); err != nil {
		return row, err
	}

	if err = setCell(file, sheetName, receiptColumnAmount, row, toSpreadsheetAmount(section.total), money.total); err != nil {
		return row, err
	}

	return row + 2, nil
}

// writeReceiptRow writes one line of the bill, and returns the row after it.
//
// A shopping trip is written as its own line with what it came to, and the positions owed off it
// beneath - the reader is told they are paying for a trip to a shop, and then which of the things
// on that trip were theirs.
func writeReceiptRow(file *excelize.File, sheetName string, styles *receiptStyles, context *DebtReceiptContext, row *receiptRow, money *receiptMoneyStyles, rowNumber int) (int, error) {
	if len(row.tripEntries) < 1 {
		return rowNumber + 1, writeReceiptEntryRow(file, sheetName, styles, context, row.entry, money, false, rowNumber)
	}

	tripName := row.tripName

	if tripName == "" {
		tripName = context.UnnamedReceiptTitle
	}

	if err := setCell(file, sheetName, receiptColumnDate, rowNumber, toSpreadsheetDate(time.Unix(row.entry.TransactionTime, 0), context.Timezone), styles.date); err != nil {
		return rowNumber, err
	}

	if err := setCell(file, sheetName, receiptColumnDescription, rowNumber, tripName, styles.tripName); err != nil {
		return rowNumber, err
	}

	if err := setCell(file, sheetName, receiptColumnAmount, rowNumber, toSpreadsheetAmount(row.tripTotal), money.trip); err != nil {
		return rowNumber, err
	}

	rowNumber++

	for i := 0; i < len(row.tripEntries); i++ {
		if err := writeReceiptEntryRow(file, sheetName, styles, context, row.tripEntries[i], money, true, rowNumber); err != nil {
			return rowNumber, err
		}

		rowNumber++
	}

	return rowNumber, nil
}

// writeReceiptEntryRow writes one thing owed.
//
// A position that belongs to a trip is written without its date and its shop, because the line
// above it has just said both, and repeating them down the page would bury the one thing that
// differs between the positions, which is what they are.
func writeReceiptEntryRow(file *excelize.File, sheetName string, styles *receiptStyles, context *DebtReceiptContext, entry *models.DebtEntryInfoResponse, money *receiptMoneyStyles, belongsToTrip bool, row int) error {
	descriptionStyle := styles.value

	if belongsToTrip {
		descriptionStyle = styles.position
	} else {
		if err := setCell(file, sheetName, receiptColumnDate, row, toSpreadsheetDate(time.Unix(entry.TransactionTime, 0), context.Timezone), styles.date); err != nil {
			return err
		}

		if err := setCell(file, sheetName, receiptColumnWhere, row, entry.MerchantName, styles.value); err != nil {
			return err
		}
	}

	if err := setCell(file, sheetName, receiptColumnDescription, row, describeEntry(entry, context), descriptionStyle); err != nil {
		return err
	}

	if err := setCell(file, sheetName, receiptColumnNote, row, entry.Comment, styles.note); err != nil {
		return err
	}

	return setCell(file, sheetName, receiptColumnAmount, row, toSpreadsheetAmount(entry.Amount), money.amount)
}

// describeEntry says what one thing owed is called.
//
// A receipt position is called what the shop called it, a transaction owed whole is called by its
// category, and a debt entered by hand is called what it was written down as - the same three names
// the debts page shows, because the person handed the receipt has to recognise what they are paying
// for.
func describeEntry(entry *models.DebtEntryInfoResponse, context *DebtReceiptContext) string {
	if entry.Name != "" {
		return entry.Name
	}

	if entry.CategoryId > 0 {
		if categoryName, exists := context.CategoryNames[entry.CategoryId]; exists && categoryName != "" {
			return categoryName
		}
	}

	return context.UnnamedTransactionTitle
}

// buildReceiptSections turns what is owed into the bills to be written, one per currency.
//
// The things owed are read oldest first, which is the order a bill is read in - the receipt is an
// account of what was bought and when, rather than the newest-first list the debts page shows.
func buildReceiptSections(entries []*models.DebtEntryInfoResponse) []*receiptSection {
	ordered := make([]*models.DebtEntryInfoResponse, len(entries))
	copy(ordered, entries)

	sortEntriesOldestFirst(ordered)

	sections := make([]*receiptSection, 0, 1)
	sectionsByCurrency := make(map[string]*receiptSection)

	for i := 0; i < len(ordered); i++ {
		section, exists := sectionsByCurrency[ordered[i].Currency]

		if !exists {
			section = &receiptSection{currency: ordered[i].Currency}
			sectionsByCurrency[ordered[i].Currency] = section
			sections = append(sections, section)
		}

		section.total += ordered[i].Amount
	}

	for i := 0; i < len(sections); i++ {
		currencyEntries := make([]*models.DebtEntryInfoResponse, 0, len(ordered))

		for j := 0; j < len(ordered); j++ {
			if ordered[j].Currency == sections[i].currency {
				currencyEntries = append(currencyEntries, ordered[j])
			}
		}

		sections[i].rows = buildReceiptRows(currencyEntries)
	}

	return sections
}

// buildReceiptRows collapses everything owed off one shopping trip into a single line with its
// positions beneath it, in the same way and for the same reason the debts page does.
//
// A trip only one thing is owed off is left as an ordinary line. There is nothing to gather, and a
// heading over a single position would only say the same thing twice.
func buildReceiptRows(entries []*models.DebtEntryInfoResponse) []*receiptRow {
	entriesByReceiptId := make(map[int64][]*models.DebtEntryInfoResponse)

	for i := 0; i < len(entries); i++ {
		if entries[i].ReceiptId <= 0 {
			continue
		}

		entriesByReceiptId[entries[i].ReceiptId] = append(entriesByReceiptId[entries[i].ReceiptId], entries[i])
	}

	rows := make([]*receiptRow, 0, len(entries))
	emittedReceiptIds := make(map[int64]bool)

	for i := 0; i < len(entries); i++ {
		receiptId := entries[i].ReceiptId

		if receiptId <= 0 || len(entriesByReceiptId[receiptId]) < 2 {
			rows = append(rows, &receiptRow{entry: entries[i]})
			continue
		}

		if emittedReceiptIds[receiptId] {
			continue
		}

		emittedReceiptIds[receiptId] = true

		receiptEntries := entriesByReceiptId[receiptId]
		tripTotal := int64(0)

		for j := 0; j < len(receiptEntries); j++ {
			tripTotal += receiptEntries[j].Amount
		}

		rows = append(rows, &receiptRow{
			entry:       receiptEntries[0],
			tripEntries: receiptEntries,
			tripName:    receiptEntries[0].MerchantName,
			tripTotal:   tripTotal,
		})
	}

	return rows
}

// sortEntriesOldestFirst orders what is owed the way a bill is read, and settles ties by id so that
// two things bought in the same second are still written in a fixed order
func sortEntriesOldestFirst(entries []*models.DebtEntryInfoResponse) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			previous := entries[j-1]
			current := entries[j]

			if previous.TransactionTime < current.TransactionTime ||
				(previous.TransactionTime == current.TransactionTime && previous.Id <= current.Id) {
				break
			}

			entries[j-1], entries[j] = current, previous
		}
	}
}

// toSpreadsheetAmount turns an amount in minor units into the number a spreadsheet holds
func toSpreadsheetAmount(amount int64) float64 {
	return float64(amount) / 100
}

// toSpreadsheetDate turns an instant into the calendar day it fell on by the user's own clock.
//
// The day is handed on as a bare date in UTC so that no clock the sheet passes through can move it
// over a midnight: what is wanted is a day of a calendar, not a moment in time.
func toSpreadsheetDate(instant time.Time, timezone *time.Location) time.Time {
	if timezone == nil {
		timezone = time.UTC
	}

	local := instant.In(timezone)

	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// buildSheetName names the tab after the person the receipt is for.
//
// A sheet name may be no longer than 31 characters and may not hold the characters a spreadsheet
// spells references with, so a name that cannot be a tab is trimmed to one rather than refused.
func buildSheetName(personName string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune("[]:*?/\\", r) {
			return -1
		}

		return r
	}, strings.TrimSpace(personName))

	if len([]rune(name)) > 31 {
		name = string([]rune(name)[:31])
	}

	name = strings.TrimSpace(name)

	if name == "" {
		return receiptDefaultSheetName
	}

	return name
}

func setCell(file *excelize.File, sheetName string, column string, row int, value any, style int) error {
	cell := fmt.Sprintf("%s%d", column, row)

	if err := file.SetCellValue(sheetName, cell, value); err != nil {
		return err
	}

	return file.SetCellStyle(sheetName, cell, cell, style)
}

func setReceiptColumnWidths(file *excelize.File, sheetName string) error {
	widths := []struct {
		column string
		width  float64
	}{
		{column: receiptColumnDate, width: 13},
		{column: receiptColumnDescription, width: 44},
		{column: receiptColumnWhere, width: 24},
		{column: receiptColumnNote, width: 30},
		{column: receiptColumnAmount, width: 15},
	}

	for i := 0; i < len(widths); i++ {
		if err := file.SetColWidth(sheetName, widths[i].column, widths[i].column, widths[i].width); err != nil {
			return err
		}
	}

	return nil
}

func buildReceiptStyles(file *excelize.File) (*receiptStyles, error) {
	styles := &receiptStyles{}

	definitions := []struct {
		target *int
		style  *excelize.Style
	}{
		{target: &styles.title, style: &excelize.Style{Font: &excelize.Font{Bold: true, Size: 16}}},
		{target: &styles.label, style: &excelize.Style{Font: &excelize.Font{Bold: true}}},
		{target: &styles.value, style: &excelize.Style{}},
		{target: &styles.valueDate, style: &excelize.Style{NumFmt: receiptShortDateNumberFormat}},
		{target: &styles.header, style: &excelize.Style{
			Font:   &excelize.Font{Bold: true},
			Border: []excelize.Border{{Type: "bottom", Color: "000000", Style: 1}},
		}},
		{target: &styles.date, style: &excelize.Style{NumFmt: receiptShortDateNumberFormat}},
		{target: &styles.tripName, style: &excelize.Style{Font: &excelize.Font{Bold: true}}},
		{target: &styles.position, style: &excelize.Style{Alignment: &excelize.Alignment{Indent: 1}}},
		{target: &styles.note, style: &excelize.Style{Font: &excelize.Font{Color: "666666"}}},
		{target: &styles.section, style: &excelize.Style{Font: &excelize.Font{Bold: true, Size: 12}}},
		{target: &styles.totalLabel, style: &excelize.Style{
			Font:      &excelize.Font{Bold: true},
			Alignment: &excelize.Alignment{Horizontal: "right"},
			Border:    []excelize.Border{{Type: "top", Color: "000000", Style: 1}},
		}},
	}

	for i := 0; i < len(definitions); i++ {
		styleId, err := file.NewStyle(definitions[i].style)

		if err != nil {
			return nil, err
		}

		*definitions[i].target = styleId
	}

	return styles, nil
}

// buildMoneyStyles makes the ways an amount is shown on a bill, all of them stating the currency it
// is counted in.
//
// The currency is written into the number's own format rather than into a column of its own, so
// that every amount says what it is while the cell stays a number that can still be added up.
func buildMoneyStyles(file *excelize.File, currency string) (*receiptMoneyStyles, error) {
	numberFormat := fmt.Sprintf("#,##0.00\" %s\"", currency)
	styles := &receiptMoneyStyles{}

	definitions := []struct {
		target *int
		style  *excelize.Style
	}{
		{target: &styles.amount, style: &excelize.Style{CustomNumFmt: &numberFormat}},
		{target: &styles.trip, style: &excelize.Style{
			CustomNumFmt: &numberFormat,
			Font:         &excelize.Font{Bold: true},
		}},
		{target: &styles.total, style: &excelize.Style{
			CustomNumFmt: &numberFormat,
			Font:         &excelize.Font{Bold: true},
			Border:       []excelize.Border{{Type: "top", Color: "000000", Style: 1}},
		}},
	}

	for i := 0; i < len(definitions); i++ {
		styleId, err := file.NewStyle(definitions[i].style)

		if err != nil {
			return nil, err
		}

		*definitions[i].target = styleId
	}

	return styles, nil
}
