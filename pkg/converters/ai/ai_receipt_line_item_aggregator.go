package ai

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
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

// maximumReceiptMerchantNameLength is the longest merchant name that can be stored with a receipt, in
// runes, matching the column it is written to
const maximumReceiptMerchantNameLength = 255

// maxReceiptRefundQuantity is the largest number of returned articles a refund line is believed to
// state. A quantity beyond this was misread, and multiplying by it would invent an amount far larger
// than anything the receipt could have printed.
const maxReceiptRefundQuantity = int64(1000)

// receiptQuantityTimesUnitPricePattern matches the "how many at what each" a receipt prints to explain
// a line, on the line itself or in the indented row under it: "-10 x 0,25", "2 x 1,49".
//
// The unit price must carry decimals, which is what keeps the pattern off a pack size printed in an
// article name ("Taschentücher 4 x 10"): a price on a German receipt always states its cents.
var receiptQuantityTimesUnitPricePattern = regexp.MustCompile(`(-?\d{1,4})\s*[xX]\s*(\d+[.,]\d{1,2})`)

// receiptLineItem is a recognized line whose price has been parsed into exact minor units
type receiptLineItem struct {
	name         string
	categoryName string
	amount       int64
	refund       bool
	// remembered records that the category was not the model's choice but the user's own, taken from
	// where they filed this article the last time they bought it
	remembered bool
}

// receiptCategoryGroupKey is what decides which lines are summed together: their category, and whether
// they are purchases or money handed back. A refund is kept apart from the purchases of the same
// category so that returning a bag of empties cannot cancel out the drinks bought on the same receipt.
type receiptCategoryGroupKey struct {
	categoryName string
	refund       bool
}

// receiptCategoryGroup holds the line items of one category while they are being aggregated
type receiptCategoryGroup struct {
	categoryName string
	itemNames    []string
	totalAmount  int64
}

// receiptRefundLineNames are the names a German receipt gives a line that hands money back rather than
// charging for a purchase - returned empties, most often. Such a line is never a discount on the item
// printed above it: the bottles have nothing to do with whatever was bought last.
//
// "Pfand" alone is not in this list and must not be, because a charged deposit is written that way.
var receiptRefundLineNames = []string{
	"rückgabe",
	"ruckgabe",
	"rücknahme",
	"rucknahme",
	"leergut",
	"retoure",
	"pfandbon",
}

// receiptNonItemLineNames are the lines a receipt prints outside its item block. The model is told not
// to transcribe them, but it routinely copies the total and the payment line anyway, and reporting
// those as lines that failed to become transactions would ask the user to book the whole receipt twice.
var receiptNonItemLineNames = []string{
	"zu zahlen",
	"summe",
	"zwischensumme",
	"gesamt",
	"gesamter preisvorteil",
	"total",
	"betrag",
	"zahlart",
	"kreditkarte",
	"ec-karte",
	"ec cash",
	"girocard",
	"maestro",
	"mastercard",
	"visa",
	"kartenzahlung",
	"bar",
	"barzahlung",
	"gegeben",
	"rückgeld",
	"wechselgeld",
	"mwst",
	"netto",
	"brutto",
	"payback",
	"deutschlandcard",
	"lidl plus",
	"punkte",
	"tse",
	"seriennr",
	"prüfwert",
	"signaturzähler",
	"ust-id",
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
func (p *aiTransactionDataParser) aggregateReceiptLineItems(c core.Context, user *models.User, result *aiTransactionDataParsedResult, accountMap map[string]*models.Account, warningCollector *converter.ImportWarningCollector, receiptCollector *converter.ImportReceiptCollector, lineItemCategories *converter.ReceiptLineItemCategoryMemory) []*models.RecognizedTransactionResult {
	accountName := p.resolveReceiptAccountName(c, user, result, accountMap)

	p.checkAllRawLinesWereItemized(c, user, result, warningCollector)

	parsedLineItems := p.parseReceiptLineItems(c, user, result.LineItems)
	applyRememberedReceiptCategories(c, user, parsedLineItems, lineItemCategories)
	reportReceiptLineItems(parsedLineItems, receiptCollector)
	p.reportReceipt(c, user, result, receiptCollector)
	groups := make([]*receiptCategoryGroup, 0, len(parsedLineItems))
	groupsByKey := make(map[receiptCategoryGroupKey]*receiptCategoryGroup, len(parsedLineItems))
	lineItemsTotalAmount := int64(0)

	for _, lineItem := range parsedLineItems {
		price := lineItem.amount
		categoryName := lineItem.categoryName
		key := receiptCategoryGroupKey{categoryName: categoryName, refund: lineItem.refund}
		group, exists := groupsByKey[key]

		if !exists {
			group = &receiptCategoryGroup{
				categoryName: categoryName,
				itemNames:    make([]string, 0, 4),
			}
			groupsByKey[key] = group
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
		// a group that came to nothing is not worth importing, but one that came to less than nothing is:
		// that is the empties handed back at the till, booked as a negative expense against the category
		// their deposit was charged to, so that the two cancel each other out over time
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
// dropping what cannot be used and folding every adjustment into the item it was charged against.
//
// A credit that belongs to the receipt rather than to any one purchase - a returned crate of empties,
// a coupon written off the whole basket - is kept as a line of its own instead. It is money the
// customer got back, so it is shown, imported and counted towards the printed total like every other
// line, rather than quietly subtracted from an unrelated purchase or dropped from the import.
func (p *aiTransactionDataParser) parseReceiptLineItems(c core.Context, user *models.User, lineItems []*models.RecognizedReceiptLineItem) []*receiptLineItem {
	parsedLineItems := make([]*receiptLineItem, 0, len(lineItems))

	// a deposit or discount belongs to the line printed directly above it, so folding it into the
	// last item only lands on the right one while nothing in between was thrown away. Once a line
	// has been dropped the item above is no longer known, and the adjustment is kept on its own
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

		// what the customer handed back at the till is never charged against the item above it, however
		// well it would fit: the bottles were bought on an earlier receipt and have nothing to do with
		// whatever was scanned last here
		if isReceiptRefundLineItem(lineItem) {
			parsedLineItems = append(parsedLineItems, &receiptLineItem{
				name:         strings.TrimSpace(lineItem.Name),
				categoryName: strings.TrimSpace(lineItem.Category),
				amount:       refundLineAmount(c, user, lineItem, price),
				refund:       true,
			})
			precedingLineDropped = false
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

			// a credit no purchase could absorb is money off the basket rather than off one item, so it
			// is kept as a line of its own - it is what the customer got back, and hiding it would leave
			// the receipt unable to add up
			//
			// a charge that found nothing to attach to is likewise kept: a deposit printed above its
			// drink, or below an item that was dropped, is still money the customer paid
			if price < 0 {
				parsedLineItems = append(parsedLineItems, &receiptLineItem{
					name:         strings.TrimSpace(lineItem.Name),
					categoryName: strings.TrimSpace(lineItem.Category),
					amount:       price,
					refund:       true,
				})
				precedingLineDropped = false
				continue
			}
		}

		parsedLineItems = append(parsedLineItems, &receiptLineItem{
			name:         strings.TrimSpace(lineItem.Name),
			categoryName: strings.TrimSpace(lineItem.Category),
			amount:       price,
		})
		precedingLineDropped = false
	}

	return parsedLineItems
}

// refundLineAmount returns what a refund line takes off the receipt, which is always a credit.
//
// When the line says how many articles were handed back and what each was worth ("Pfandrückgabe
// -10 x 0,25"), that is what the line comes to, and multiplying the two is arithmetic the server owns.
// The model reads those two numbers off one printed row reliably; which of the three numbers on that
// row is the line total is what it gets wrong, taking the 0,25 unit price where -2,50 is printed.
//
// Failing that, only the sign can be repaired: a line that hands money back cannot be a charge, and
// letting a positive one through would add money to the receipt the customer never paid. How far off
// its size is then left to the total check.
func refundLineAmount(c core.Context, user *models.User, lineItem *models.RecognizedReceiptLineItem, price int64) int64 {
	if amount, ok := refundAmountFromQuantity(lineItem.Name); ok {
		if amount != price {
			log.Warnf(c, "[ai_receipt_line_item_aggregator.refundLineAmount] receipt refund \"%s\" of user \"uid:%d\" was read as \"%s\", using the %s its own quantity comes to instead", lineItem.Name, user.Uid, lineItem.Price, utils.FormatAmount(amount))
		}

		return amount
	}

	if price <= 0 {
		return price
	}

	log.Warnf(c, "[ai_receipt_line_item_aggregator.refundLineAmount] receipt refund \"%s\" of user \"uid:%d\" was read as a positive price \"%s\", booking it as a credit instead", lineItem.Name, user.Uid, lineItem.Price)

	return -price
}

// refundAmountFromQuantity returns what a refund line comes to when its name states how many articles
// were handed back at what each was worth, and whether it stated that at all
func refundAmountFromQuantity(name string) (int64, bool) {
	match := receiptQuantityTimesUnitPricePattern.FindStringSubmatch(name)

	if match == nil {
		return 0, false
	}

	quantity, err := strconv.ParseInt(match[1], 10, 64)

	if err != nil {
		return 0, false
	}

	if quantity < 0 {
		quantity = -quantity
	}

	if quantity < 1 || quantity > maxReceiptRefundQuantity {
		return 0, false
	}

	unitPrice, err := parseReceiptAmount(match[2])

	if err != nil || unitPrice <= 0 {
		return 0, false
	}

	return -quantity * unitPrice, true
}

// applyRememberedReceiptCategories files each line under the category the user put it in the last
// time they bought it, wherever they have bought it before.
//
// This runs after the lines have been parsed and before they are grouped, so it acts on exactly the
// lines the user is shown and can drag - a deposit already folded into its drink is not looked up on
// its own, and cannot be filed anywhere the drink is not.
//
// The user's own answer beats the model's every time it exists. The model is guessing from an article
// name and a list of category names; the user is stating what they decided about this very article,
// and they decided it by looking at the receipt. Where they have said nothing, the model's choice is
// left alone.
func applyRememberedReceiptCategories(c core.Context, user *models.User, parsedLineItems []*receiptLineItem, lineItemCategories *converter.ReceiptLineItemCategoryMemory) {
	if lineItemCategories.Len() < 1 || len(parsedLineItems) < 1 {
		return
	}

	rememberedCount := 0
	correctedCount := 0

	for _, lineItem := range parsedLineItems {
		if lineItem.name == "" {
			continue
		}

		categoryName, exists := lineItemCategories.FindCategoryName(lineItem.name)

		if !exists {
			continue
		}

		rememberedCount++

		if categoryName != lineItem.categoryName {
			correctedCount++
			lineItem.categoryName = categoryName
		}

		lineItem.remembered = true
	}

	if rememberedCount > 0 {
		log.Infof(c, "[ai_receipt_line_item_aggregator.applyRememberedReceiptCategories] filed %d of %d receipt lines of user \"uid:%d\" under a category remembered from an earlier receipt, %d of which the model had put elsewhere", rememberedCount, len(parsedLineItems), user.Uid, correctedCount)
	}
}

// reportReceiptLineItems hands the parsed lines back to the caller, in the order they are printed on
// the receipt, so that the import UI can show what each transaction was summed from and let the user
// move a line the model filed under the wrong category.
//
// The lines are reported as they are after parsing, with every deposit and discount already charged
// against the item above it, so that a line the user sees is a purchase they can point at on the
// receipt and the lines of a category always add up to that category's transaction.
// reportReceipt hands back what the receipt states about itself as a whole - where the shopping was
// done and what the till printed as the total - so that the import can record the shopping trip its
// transactions belong to.
//
// The printed total is reported whether or not it agrees with the lines. Where it does not, the
// disagreement is already a warning the user is shown, and keeping the receipt's own claim is what
// lets that question still be answered later.
func (p *aiTransactionDataParser) reportReceipt(c core.Context, user *models.User, result *aiTransactionDataParsedResult, receiptCollector *converter.ImportReceiptCollector) {
	if receiptCollector == nil {
		return
	}

	receiptCollector.SetMerchantName(truncateReceiptMerchantName(strings.TrimSpace(result.Merchant)))

	receiptTotal := strings.TrimSpace(result.ReceiptTotal)

	if receiptTotal == "" {
		return
	}

	printedTotal, err := parseReceiptAmount(receiptTotal)

	if err != nil {
		log.Warnf(c, "[ai_receipt_line_item_aggregator.reportReceipt] cannot record the printed total of the receipt of user \"uid:%d\", because \"%s\" cannot be parsed", user.Uid, receiptTotal)
		return
	}

	receiptCollector.SetPrintedTotal(printedTotal)
}

// truncateReceiptMerchantName cuts a merchant name to what the column can hold. A till prints a shop
// name in a few words, so this only ever fires on a model that answered with the whole receipt header.
func truncateReceiptMerchantName(merchantName string) string {
	runes := []rune(merchantName)

	if len(runes) <= maximumReceiptMerchantNameLength {
		return merchantName
	}

	return string(runes[:maximumReceiptMerchantNameLength])
}

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
			Refund:       lineItem.refund,
			Remembered:   lineItem.remembered,
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

// isReceiptRefundLineItem reports whether a line hands money back to the customer instead of charging
// them - "Pfandrückgabe", "Leergut", a returned article. Such a line is recognized by its name alone,
// because what it is has to be decided before its price is trusted: a "Pfandrückgabe" the model read as
// a positive 0,25 is still a refund, and treating it as a deposit would add it onto the item above it.
func isReceiptRefundLineItem(lineItem *models.RecognizedReceiptLineItem) bool {
	name := strings.ToLower(lineItem.Name)

	for _, refundName := range receiptRefundLineNames {
		if strings.Contains(name, refundName) {
			return true
		}
	}

	return false
}

// isReceiptNonItemLine reports whether a transcribed line is one of the receipt's own summary lines
// rather than something that was bought - the total, the payment line, the VAT table.
//
// The name has to start the line for it to count: a receipt line is labelled at its left, while an
// article whose name merely contains one of these words ("Barilla", "Visabella") is a purchase like
// any other. The character after the name must not be a letter either, so that "Bar" does not claim
// "Barilla" while "Bar 20,00" and "Summe:" are both still recognized.
func isReceiptNonItemLine(rawLine string) bool {
	line := normalizeReceiptText(rawLine)

	if isReceiptVatTableLine(line) {
		return true
	}

	for _, nonItemName := range receiptNonItemLineNames {
		if !strings.HasPrefix(line, nonItemName) {
			continue
		}

		remainder := line[len(nonItemName):]

		if remainder == "" {
			return true
		}

		if firstRune := []rune(remainder)[0]; !unicode.IsLetter(firstRune) {
			return true
		}
	}

	return false
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

	// the transcript is asked for the item block alone, but models routinely copy the total and the
	// payment line along with it. Those were never meant to become transactions, and reporting them as
	// lines that failed to would tell the user to book the whole receipt a second time by hand.
	transcribedItemLines := filterReceiptItemLines(result.RawLines)

	if len(transcribedItemLines) <= len(result.LineItems) {
		return
	}

	missingLineCount := len(transcribedItemLines) - len(result.LineItems)
	missingLines := findUnitemizedRawLines(transcribedItemLines, result.LineItems)

	log.Warnf(c, "[ai_receipt_line_item_aggregator.checkAllRawLinesWereItemized] the model transcribed %d receipt lines, %d of them item lines, but only itemized %d for user \"uid:%d\", so %d printed line(s) were lost: %s", len(result.RawLines), len(transcribedItemLines), len(result.LineItems), user.Uid, missingLineCount, strings.Join(result.RawLines, " | "))

	warningCollector.Add(&models.ImportTransactionWarningResponse{
		Type:          models.IMPORT_TRANSACTION_WARNING_RECEIPT_LINES_NOT_ITEMIZED,
		LineItemCount: missingLineCount,
		MissingLines:  missingLines,
	})
}

// isReceiptVatTableLine reports whether a normalized line is a row of the VAT summary table, which is
// labelled by the VAT rate marker of the lines it sums up ("A  7 %  3,01  42,94  45,95").
//
// The rate marker alone would be far too little to go on, so the percentage is required as well: an
// article can be named "A..." and another can be sold at "45% Fett i.Tr.", but a line that starts with
// a bare A or B and states a percentage is the tax table and nothing else.
func isReceiptVatTableLine(line string) bool {
	if !strings.HasPrefix(line, "a ") && !strings.HasPrefix(line, "b ") {
		return false
	}

	return strings.Contains(line, "%")
}

// filterReceiptItemLines keeps the transcribed lines that could be a purchase, dropping the receipt's
// own summary lines. Order is preserved, because the caller pairs what is left against the recognized
// items in the order both were printed.
func filterReceiptItemLines(rawLines []string) []string {
	itemLines := make([]string, 0, len(rawLines))

	for _, rawLine := range rawLines {
		if isReceiptNonItemLine(rawLine) {
			continue
		}

		itemLines = append(itemLines, rawLine)
	}

	return itemLines
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
