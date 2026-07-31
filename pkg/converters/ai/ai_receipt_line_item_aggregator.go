package ai

import (
	"strings"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/core"
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

// receiptCategoryGroup holds the line items of one category while they are being aggregated
type receiptCategoryGroup struct {
	categoryName string
	itemNames    []string
	totalAmount  int64
}

// aggregateReceiptLineItems groups the recognized receipt line items by their category and sums each group,
// producing one expense transaction per category. The large language model is only asked to read and
// categorize the individual lines, all arithmetic is done here with exact minor unit integers.
func (p *aiTransactionDataParser) aggregateReceiptLineItems(c core.Context, user *models.User, result *aiTransactionDataParsedResult, warningCollector *converter.ImportWarningCollector) []*models.RecognizedTransactionResult {
	groups := make([]*receiptCategoryGroup, 0, len(result.LineItems))
	groupsByCategoryName := make(map[string]*receiptCategoryGroup, len(result.LineItems))
	lineItemsTotalAmount := int64(0)
	aggregatedLineItemCount := 0

	for _, lineItem := range result.LineItems {
		if lineItem == nil {
			continue
		}

		price, err := utils.ParseAmount(strings.TrimSpace(lineItem.Price))

		if err != nil {
			log.Warnf(c, "[ai_receipt_line_item_aggregator.aggregateReceiptLineItems] skipping receipt line item \"%s\" for user \"uid:%d\", because its price \"%s\" cannot be parsed", lineItem.Name, user.Uid, lineItem.Price)
			continue
		}

		if price < 0 {
			log.Warnf(c, "[ai_receipt_line_item_aggregator.aggregateReceiptLineItems] skipping receipt line item \"%s\" for user \"uid:%d\", because its price \"%s\" is negative", lineItem.Name, user.Uid, lineItem.Price)
			continue
		}

		categoryName := strings.TrimSpace(lineItem.Category)
		group, exists := groupsByCategoryName[categoryName]

		if !exists {
			group = &receiptCategoryGroup{
				categoryName: categoryName,
				itemNames:    make([]string, 0, 4),
			}
			groupsByCategoryName[categoryName] = group
			groups = append(groups, group)
		}

		itemName := strings.TrimSpace(lineItem.Name)

		if itemName != "" {
			group.itemNames = append(group.itemNames, itemName)
		}

		group.totalAmount += price
		lineItemsTotalAmount += price
		aggregatedLineItemCount++
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
			AccountName:  result.AccountName,
			CategoryName: group.categoryName,
			Description:  truncateTransactionDescription(strings.Join(group.itemNames, ", ")),
		})
	}

	p.checkReceiptTotal(c, user, result.ReceiptTotal, lineItemsTotalAmount, aggregatedLineItemCount, warningCollector)

	return transactions
}

// checkReceiptTotal compares the sum of the recognized line items against the total printed on the receipt
// and reports a warning when they differ by at least the mismatch threshold. A mismatch never fails the
// import, the user can still correct the transactions before importing them.
func (p *aiTransactionDataParser) checkReceiptTotal(c core.Context, user *models.User, receiptTotal string, lineItemsTotalAmount int64, lineItemCount int, warningCollector *converter.ImportWarningCollector) {
	receiptTotal = strings.TrimSpace(receiptTotal)

	if receiptTotal == "" {
		return
	}

	statedTotalAmount, err := utils.ParseAmount(receiptTotal)

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
