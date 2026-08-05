package ai

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// receiptTotalMismatchThreshold is the smallest difference between the sum of the recognized line items
// and the total printed on the receipt that is worth reporting, in minor units (0.50)
const receiptTotalMismatchThreshold = int64(50)

// maxTransactionDescriptionLength is the maximum length of a transaction description accepted by the
// transaction model, longer descriptions are truncated instead of failing the import
const maxTransactionDescriptionLength = 255

// receiptLineItem is a recognized line whose price has been parsed into exact minor units
type receiptLineItem struct {
	name         string
	categoryName string
	amount       int64
}

// receiptCategoryGroup holds the line items of one category while they are being aggregated
type receiptCategoryGroup struct {
	categoryName string
	itemNames    []string
	totalAmount  int64
}

// how the payment method printed on the receipt maps onto an account category, so that a receipt
// paid in cash is booked against a cash account and a card payment against the matching card
// account. Card payments list both card categories: plenty of people keep a single account for
// every card they own, so an exact category match must not be the only thing that works.
var receiptPaymentMethodAccountCategories = map[string][]models.AccountCategory{
	"cash":   {models.ACCOUNT_CATEGORY_CASH},
	"debit":  {models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT, models.ACCOUNT_CATEGORY_CREDIT_CARD},
	"credit": {models.ACCOUNT_CATEGORY_CREDIT_CARD, models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT},
}

// aggregateReceiptLineItems groups the recognized receipt line items by their category and sums each group,
// producing one expense transaction per category. The large language model is only asked to read and
// categorize the individual lines, all arithmetic is done here with exact minor unit integers.
func (p *aiTransactionDataParser) aggregateReceiptLineItems(c core.Context, user *models.User, result *aiTransactionDataParsedResult, accountMap map[string]*models.Account, warningCollector *converter.ImportWarningCollector, receiptCollector *converter.ImportReceiptCollector) []*models.RecognizedTransactionResult {
	accountName := p.resolveReceiptAccountName(c, user, result, accountMap)

	p.checkAllRawLinesWereItemized(c, user, result, warningCollector)

	parsedLineItems, unallocatedAdjustments := p.parseReceiptLineItems(c, user, result.LineItems)
	reportReceiptLineItems(parsedLineItems, receiptCollector)
	groups := make([]*receiptCategoryGroup, 0, len(parsedLineItems))
	groupsByCategoryName := make(map[string]*receiptCategoryGroup, len(parsedLineItems))

	// credits that belong to the receipt rather than to any one purchase are not imported, but the
	// receipt still totals to less because of them, so they count here
	lineItemsTotalAmount := unallocatedAdjustments

	for _, lineItem := range parsedLineItems {
		price := lineItem.amount
		categoryName := lineItem.categoryName
		group, exists := groupsByCategoryName[categoryName]

		if !exists {
			group = &receiptCategoryGroup{
				categoryName: categoryName,
				itemNames:    make([]string, 0, 4),
			}
			groupsByCategoryName[categoryName] = group
			groups = append(groups, group)
		}

		if lineItem.name != "" {
			group.itemNames = append(group.itemNames, lineItem.name)
		}

		group.totalAmount += price
		lineItemsTotalAmount += price
	}

	transactions := make([]*models.RecognizedTransactionResult, 0, len(groups))

	for _, group := range groups {
		// no item is ever negative, because a discount is only charged to a line large enough to absorb
		// it, so a group is either a real expense or one that came to nothing and is not worth importing
		if group.totalAmount <= 0 {
			continue
		}

		transactions = append(transactions, &models.RecognizedTransactionResult{
			Type:         "expense",
			Time:         result.Time,
			Amount:       utils.FormatAmount(group.totalAmount),
			AccountName:  accountName,
			CategoryName: group.categoryName,
			Description:  truncateTransactionDescription(strings.Join(group.itemNames, ", ")),
		})
	}

	p.checkReceiptTotal(c, user, result.ReceiptTotal, lineItemsTotalAmount, len(parsedLineItems), warningCollector)

	return transactions
}

// parseReceiptLineItems turns the recognized lines into parsed items with exact minor unit amounts,
// dropping what cannot be used and folding every adjustment into the item it was charged against.
//
// It also returns the adjustments that no single item could absorb - a returned crate of empties, a
// coupon written off the whole basket. Those belong to the receipt rather than to any one purchase,
// so they never become a transaction, but they are still part of what the receipt totals to and the
// caller counts them when it checks the sum against the printed total.
func (p *aiTransactionDataParser) parseReceiptLineItems(c core.Context, user *models.User, lineItems []*models.RecognizedReceiptLineItem) ([]*receiptLineItem, int64) {
	parsedLineItems := make([]*receiptLineItem, 0, len(lineItems))
	unallocatedAdjustments := int64(0)

	// a deposit or discount belongs to the line printed directly above it, so folding it into the
	// last item only lands on the right one while nothing in between was thrown away. Once a line
	// has been dropped the item above is no longer known, and the adjustment is left unallocated
	// instead of being quietly charged to an unrelated purchase.
	precedingLineDropped := false

	for _, lineItem := range lineItems {
		if lineItem == nil {
			continue
		}

		price, err := parseReceiptAmount(lineItem.Price)

		if err != nil {
			log.Warnf(c, "[ai_receipt_line_item_aggregator.parseReceiptLineItems] skipping receipt line item \"%s\" for user \"uid:%d\", because its price \"%s\" cannot be parsed", lineItem.Name, user.Uid, lineItem.Price)
			precedingLineDropped = true
			continue
		}

		// a deposit is not a purchase of its own and a discount is not a purchase at all: both are
		// part of what the item above them cost, so they are added to that item (a discount with its
		// negative price subtracting itself) and never named separately. The category is inherited
		// too, which also repairs an adjustment the model filed under the wrong one.
		if isReceiptAdjustmentLineItem(lineItem, price) {
			if p.chargeReceiptAdjustment(c, user, parsedLineItems, lineItem, price, precedingLineDropped) {
				continue
			}

			// a credit that found nothing to reduce is not a purchase, so it stays out of the import
			// and only counts towards the receipt total
			if price < 0 {
				unallocatedAdjustments += price
				continue
			}

			// a charge that found nothing to attach to is still money the customer paid - a deposit
			// printed above its drink, or below an item that was dropped - so it is kept as a line
			// item of its own rather than dropped
		}

		parsedLineItems = append(parsedLineItems, &receiptLineItem{
			name:         strings.TrimSpace(lineItem.Name),
			categoryName: strings.TrimSpace(lineItem.Category),
			amount:       price,
		})
		precedingLineDropped = false
	}

	return parsedLineItems, unallocatedAdjustments
}

// reportReceiptLineItems hands the parsed lines back to the caller, in the order they are printed on
// the receipt, so that the import UI can show what each transaction was summed from and let the user
// move a line the model filed under the wrong category.
//
// The lines are reported as they are after parsing, with every deposit and discount already charged
// against the item above it, so that a line the user sees is a purchase they can point at on the
// receipt and the lines of a category always add up to that category's transaction.
func reportReceiptLineItems(parsedLineItems []*receiptLineItem, receiptCollector *converter.ImportReceiptCollector) {
	if receiptCollector == nil || len(parsedLineItems) < 1 {
		return
	}

	lineItems := make([]*models.ImportReceiptLineItemResponse, 0, len(parsedLineItems))

	for _, lineItem := range parsedLineItems {
		lineItems = append(lineItems, &models.ImportReceiptLineItemResponse{
			Name:         lineItem.name,
			Amount:       lineItem.amount,
			CategoryName: lineItem.categoryName,
		})
	}

	receiptCollector.Set(lineItems)
}

// chargeReceiptAdjustment adds a deposit or discount onto the item it was printed under and reports
// whether that item could take it.
//
// An adjustment is only charged to an item that can absorb it. A credit bigger than the line above it
// is not a discount on that line - it is a refund for something else entirely, most often a crate of
// empties handed in at the till - and subtracting it there would turn a real purchase into a negative
// amount and lose it from the import along with the credit.
func (p *aiTransactionDataParser) chargeReceiptAdjustment(c core.Context, user *models.User, parsedLineItems []*receiptLineItem, lineItem *models.RecognizedReceiptLineItem, price int64, precedingLineDropped bool) bool {
	if len(parsedLineItems) < 1 {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.chargeReceiptAdjustment] leaving receipt adjustment \"%s\" (%s) of user \"uid:%d\" unallocated, because no item precedes it to charge it against", lineItem.Name, lineItem.Price, user.Uid)
		return false
	}

	previousLineItem := parsedLineItems[len(parsedLineItems)-1]

	if previousLineItem.amount+price < 0 {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.chargeReceiptAdjustment] leaving receipt adjustment \"%s\" (%s) of user \"uid:%d\" unallocated, because the item above it (\"%s\", %s) is not large enough to absorb it", lineItem.Name, lineItem.Price, user.Uid, previousLineItem.name, utils.FormatAmount(previousLineItem.amount))
		return false
	}

	if precedingLineDropped {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.chargeReceiptAdjustment] charging receipt adjustment \"%s\" (%s) of user \"uid:%d\" against \"%s\", which may not be the item it was printed under, because the line between them was dropped", lineItem.Name, lineItem.Price, user.Uid, previousLineItem.name)
	}

	previousLineItem.amount += price

	return true
}

// isReceiptAdjustmentLineItem reports whether a line adjusts what the item above it cost rather than
// being a purchase of its own - a charged deposit, or a discount printed as its own negative line.
//
// Deposits are flagged by the model, and the name is checked as well because a missing flag would
// put a "Pfand 0,25 EM" entry back into the description. Discounts are recognized by their negative
// price alone: German receipts name them a dozen ways ("Rabatt", "Nachlass", "Preisvorteil",
// "Coupon", a bare "Aktion"), and the minus sign is the one thing they all share.
func isReceiptAdjustmentLineItem(lineItem *models.RecognizedReceiptLineItem, price int64) bool {
	if price < 0 {
		return true
	}

	return lineItem.Deposit || strings.Contains(strings.ToLower(lineItem.Name), "pfand")
}

// resolveReceiptAccountName returns the account every transaction of this receipt is booked against.
// The payment method read off the receipt wins over the account name the model picked from the list:
// the payment line is evidence printed on the receipt, the account name is only a guess. The guess is
// still used when the payment method is missing, or when it does not identify exactly one account.
func (p *aiTransactionDataParser) resolveReceiptAccountName(c core.Context, user *models.User, result *aiTransactionDataParsedResult, accountMap map[string]*models.Account) string {
	paymentMethod := strings.ToLower(strings.TrimSpace(result.PaymentMethod))

	if paymentMethod == "" || len(accountMap) < 1 {
		return result.AccountName
	}

	accountCategories, exists := receiptPaymentMethodAccountCategories[paymentMethod]

	if !exists {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.resolveReceiptAccountName] unknown payment method \"%s\" for user \"uid:%d\"", result.PaymentMethod, user.Uid)
		return result.AccountName
	}

	for _, accountCategory := range accountCategories {
		candidateNames := make([]string, 0, len(accountMap))

		for accountName, account := range accountMap {
			if account != nil && account.Category == accountCategory {
				candidateNames = append(candidateNames, accountName)
			}
		}

		if len(candidateNames) < 1 {
			continue
		}

		// with several accounts of the same kind the model may still have named the right one,
		// e.g. it read "VISA" off the receipt and the user has both a Visa and a Mastercard
		for _, candidateName := range candidateNames {
			if candidateName == result.AccountName {
				return candidateName
			}
		}

		if len(candidateNames) == 1 {
			return candidateNames[0]
		}

		sort.Strings(candidateNames)
		log.Warnf(c, "[ai_receipt_line_item_aggregator.resolveReceiptAccountName] payment method \"%s\" of user \"uid:%d\" matches several accounts (%s), falling back to the recognized account name \"%s\"", paymentMethod, user.Uid, strings.Join(candidateNames, ", "), result.AccountName)

		return result.AccountName
	}

	log.Warnf(c, "[ai_receipt_line_item_aggregator.resolveReceiptAccountName] payment method \"%s\" of user \"uid:%d\" matches no account, falling back to the recognized account name \"%s\"", paymentMethod, user.Uid, result.AccountName)

	return result.AccountName
}

// checkAllRawLinesWereItemized reports the printed lines the model transcribed but never turned into a
// line item. The two stage prompt exists precisely so that a dropped line leaves a trace: stage 1 copies
// the receipt out verbatim, stage 2 interprets that copy, and anything present in the first list and
// missing from the second is money that would otherwise disappear from the import without a word.
//
// A total mismatch alone cannot replace this - it says the sum is wrong but not which line is gone, and
// it stays silent when the lost lines happen to fall under the mismatch threshold.
func (p *aiTransactionDataParser) checkAllRawLinesWereItemized(c core.Context, user *models.User, result *aiTransactionDataParsedResult, warningCollector *converter.ImportWarningCollector) {
	// a model that answers without the transcript leaves nothing to compare the items against, so
	// this check cannot run at all. That is worth saying out loud: silence here would otherwise read
	// as "no line was lost" when it actually means "nobody looked".
	if len(result.RawLines) < 1 {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.checkAllRawLinesWereItemized] the model returned %d receipt line item(s) for user \"uid:%d\" but no transcript, so lines it dropped before itemizing them cannot be detected", len(result.LineItems), user.Uid)
		return
	}

	if len(result.RawLines) <= len(result.LineItems) {
		return
	}

	missingLineCount := len(result.RawLines) - len(result.LineItems)
	missingLines := findUnitemizedRawLines(result.RawLines, result.LineItems)

	log.Warnf(c, "[ai_receipt_line_item_aggregator.checkAllRawLinesWereItemized] the model transcribed %d receipt lines but only itemized %d of them for user \"uid:%d\", so %d printed line(s) were lost: %s", len(result.RawLines), len(result.LineItems), user.Uid, missingLineCount, strings.Join(result.RawLines, " | "))

	warningCollector.Add(&models.ImportTransactionWarningResponse{
		Type:          models.IMPORT_TRANSACTION_WARNING_RECEIPT_LINES_NOT_ITEMIZED,
		LineItemCount: missingLineCount,
		MissingLines:  missingLines,
	})
}

// findUnitemizedRawLines pairs the transcribed lines against the recognized items and returns the lines
// that never got one. Both lists are in receipt order, so a single walk pairs them up: a transcribed
// line belongs to the next item still waiting when it contains that item's name.
//
// The pairing is only reported when it consumed every line item. If it did not, the model renamed
// something on its way from one stage to the other and the leftovers would name innocent lines, so
// nothing is returned and the caller falls back to reporting the count alone.
func findUnitemizedRawLines(rawLines []string, lineItems []*models.RecognizedReceiptLineItem) []string {
	unitemizedLines := make([]string, 0, len(rawLines)-len(lineItems))
	itemIndex := 0

	for _, rawLine := range rawLines {
		if itemIndex < len(lineItems) && rawLineMatchesLineItem(rawLine, lineItems[itemIndex]) {
			itemIndex++
			continue
		}

		unitemizedLines = append(unitemizedLines, rawLine)
	}

	if itemIndex < len(lineItems) {
		return nil
	}

	return unitemizedLines
}

// rawLineMatchesLineItem reports whether a transcribed line is the one a recognized item came from,
// which it is when the printed line still contains the item's name. Whitespace is collapsed because
// receipts pad the gap between name and price, and the comparison ignores case.
func rawLineMatchesLineItem(rawLine string, lineItem *models.RecognizedReceiptLineItem) bool {
	if lineItem == nil {
		return false
	}

	name := normalizeReceiptText(lineItem.Name)

	if name == "" {
		return false
	}

	return strings.Contains(normalizeReceiptText(rawLine), name)
}

// normalizeReceiptText lowercases a receipt line and collapses every run of whitespace into a single
// space, so that the padding a receipt printer inserts does not decide whether two texts match
func normalizeReceiptText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// checkReceiptTotal compares the sum of the recognized line items against the total printed on the receipt
// and reports a warning when they differ by at least the mismatch threshold. A mismatch never fails the
// import, the user can still correct the transactions before importing them.
func (p *aiTransactionDataParser) checkReceiptTotal(c core.Context, user *models.User, receiptTotal string, lineItemsTotalAmount int64, lineItemCount int, warningCollector *converter.ImportWarningCollector) {
	receiptTotal = strings.TrimSpace(receiptTotal)

	if receiptTotal == "" {
		return
	}

	statedTotalAmount, err := parseReceiptAmount(receiptTotal)

	if err != nil {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.checkReceiptTotal] cannot validate the recognized receipt line items for user \"uid:%d\", because the receipt total \"%s\" cannot be parsed", user.Uid, receiptTotal)
		return
	}

	difference := lineItemsTotalAmount - statedTotalAmount

	if difference == 0 {
		return
	}

	absoluteDifference := difference

	if absoluteDifference < 0 {
		absoluteDifference = -absoluteDifference
	}

	if absoluteDifference < receiptTotalMismatchThreshold {
		return
	}

	log.Warnf(c, "[ai_receipt_line_item_aggregator.checkReceiptTotal] the recognized receipt line items of user \"uid:%d\" sum up to %s but the receipt total is %s, difference is %s", user.Uid, utils.FormatAmount(lineItemsTotalAmount), utils.FormatAmount(statedTotalAmount), utils.FormatAmount(difference))

	warningCollector.Add(&models.ImportTransactionWarningResponse{
		Type:            models.IMPORT_TRANSACTION_WARNING_RECEIPT_TOTAL_MISMATCH,
		LineItemCount:   lineItemCount,
		CalculatedTotal: utils.FormatAmount(lineItemsTotalAmount),
		StatedTotal:     utils.FormatAmount(statedTotalAmount),
		Difference:      utils.FormatAmount(difference),
	})
}

// parseReceiptAmount parses an amount the way it may appear on a German receipt. The prompt asks
// the model for a plain "1234.56", but small models routinely echo the receipt verbatim ("32,79",
// "1.234,56") and those would otherwise be dropped as unparseable - silently losing money off the
// receipt. Normalising here rather than relying on the prompt keeps that from happening.
func parseReceiptAmount(amount string) (int64, error) {
	var builder strings.Builder
	hasDigit := false

	for _, char := range amount {
		if char >= '0' && char <= '9' {
			hasDigit = true
		} else if char != ',' && char != '.' && char != '-' && char != '+' {
			continue
		}

		builder.WriteRune(char)
	}

	// utils.ParseAmount treats an empty string as zero, so text with no digit in it at all has to
	// be rejected here - otherwise an unreadable price would silently become 0.00
	if !hasDigit {
		return 0, errs.ErrNumberInvalid
	}

	normalized := builder.String()
	lastComma := strings.LastIndex(normalized, ",")
	lastDot := strings.LastIndex(normalized, ".")

	if lastComma >= 0 && lastDot >= 0 {
		// whichever separator comes last is the decimal one, the other groups thousands
		if lastComma > lastDot {
			normalized = strings.ReplaceAll(normalized, ".", "")
		} else {
			normalized = strings.ReplaceAll(normalized, ",", "")
		}
	}

	// any comma left is the decimal separator, earlier ones grouped thousands
	if lastComma = strings.LastIndex(normalized, ","); lastComma >= 0 {
		normalized = strings.ReplaceAll(normalized[:lastComma], ",", "") + "." + normalized[lastComma+1:]
	}

	return utils.ParseAmount(normalized)
}

// truncateTransactionDescription shortens the joined item names to the maximum length accepted by the
// transaction model, cutting on an item boundary where possible
func truncateTransactionDescription(description string) string {
	if utf8.RuneCountInString(description) <= maxTransactionDescriptionLength {
		return description
	}

	truncated := string([]rune(description)[:maxTransactionDescriptionLength-1])

	if lastSeparatorIndex := strings.LastIndex(truncated, ", "); lastSeparatorIndex > 0 {
		truncated = truncated[:lastSeparatorIndex]
	}

	return truncated + "…"
}
