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
func (p *aiTransactionDataParser) aggregateReceiptLineItems(c core.Context, user *models.User, result *aiTransactionDataParsedResult, accountMap map[string]*models.Account, warningCollector *converter.ImportWarningCollector) []*models.RecognizedTransactionResult {
	accountName := p.resolveReceiptAccountName(c, user, result, accountMap)

	if len(result.RawLines) > len(result.LineItems) {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.aggregateReceiptLineItems] the model transcribed %d receipt lines but only itemized %d of them for user \"uid:%d\", so %d printed line(s) were lost: %s", len(result.RawLines), len(result.LineItems), user.Uid, len(result.RawLines)-len(result.LineItems), strings.Join(result.RawLines, " | "))
	}

	parsedLineItems := p.parseReceiptLineItems(c, user, result.LineItems)
	groups := make([]*receiptCategoryGroup, 0, len(parsedLineItems))
	groupsByCategoryName := make(map[string]*receiptCategoryGroup, len(parsedLineItems))
	lineItemsTotalAmount := int64(0)

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
		if group.totalAmount == 0 {
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
// dropping what cannot be used and folding each deposit into the item it was charged on.
func (p *aiTransactionDataParser) parseReceiptLineItems(c core.Context, user *models.User, lineItems []*models.RecognizedReceiptLineItem) []*receiptLineItem {
	parsedLineItems := make([]*receiptLineItem, 0, len(lineItems))

	for _, lineItem := range lineItems {
		if lineItem == nil {
			continue
		}

		price, err := parseReceiptAmount(lineItem.Price)

		if err != nil {
			log.Warnf(c, "[ai_receipt_line_item_aggregator.parseReceiptLineItems] skipping receipt line item \"%s\" for user \"uid:%d\", because its price \"%s\" cannot be parsed", lineItem.Name, user.Uid, lineItem.Price)
			continue
		}

		if price < 0 {
			log.Warnf(c, "[ai_receipt_line_item_aggregator.parseReceiptLineItems] skipping receipt line item \"%s\" for user \"uid:%d\", because its price \"%s\" is negative", lineItem.Name, user.Uid, lineItem.Price)
			continue
		}

		// a deposit is not a purchase of its own, it is part of what the drink above it cost, so it
		// is added to that item and never named separately. Its category is inherited too, which
		// also repairs a deposit the model filed under the wrong category.
		if isReceiptDepositLineItem(lineItem) && len(parsedLineItems) > 0 {
			previousLineItem := parsedLineItems[len(parsedLineItems)-1]
			previousLineItem.amount += price
			continue
		}

		parsedLineItems = append(parsedLineItems, &receiptLineItem{
			name:         strings.TrimSpace(lineItem.Name),
			categoryName: strings.TrimSpace(lineItem.Category),
			amount:       price,
		})
	}

	return parsedLineItems
}

// isReceiptDepositLineItem reports whether a line is a deposit charged on the item above it. The
// model is asked to flag these, and the name is checked as well because a missing flag would put a
// "Pfand 0,25 EM" entry back into the description.
func isReceiptDepositLineItem(lineItem *models.RecognizedReceiptLineItem) bool {
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
