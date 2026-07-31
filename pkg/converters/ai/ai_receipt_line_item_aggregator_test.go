package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// the Lidl Berlin receipt of 2026-07-28, 14 purchased lines, 32.79 EUR in total
func createTestReceiptLineItems() []*models.RecognizedReceiptLineItem {
	return []*models.RecognizedReceiptLineItem{
		{Name: "Broccoli", Price: "1.49", Category: "Food"},
		{Name: "Kartoffeln früh", Price: "2.99", Category: "Food"},
		{Name: "Strauchtomaten", Price: "1.69", Category: "Food"},
		{Name: "Bio Möhren", Price: "1.89", Category: "Food"},
		{Name: "Kiwi", Price: "2.29", Category: "Fruit & Snack"},
		{Name: "Heidelbeeren", Price: "2.19", Category: "Fruit & Snack"},
		{Name: "Frischer O-Saft o.F.", Price: "2.59", Category: "Drink"},
		{Name: "Pfand 0,25 EM", Price: "0.25", Deposit: true, Category: "Drink"},
		{Name: "Monster Mango Loco", Price: "1.49", Category: "Drink"},
		{Name: "Pfand 0,25 M", Price: "0.25", Deposit: true, Category: "Drink"},
		{Name: "Milchcreme Cookies", Price: "1.49", Category: "Food"},
		{Name: "Papiertragetasche", Price: "0.20", Category: "Houseware"},
		{Name: "Akku Ni-MH-0532273", Price: "6.99", Category: "Electronics"},
		{Name: "Akku Ni-MH-0532273", Price: "6.99", Category: "Electronics"},
	}
}

func aggregateTestLineItems(result *aiTransactionDataParsedResult) ([]*models.RecognizedTransactionResult, []*models.ImportTransactionWarningResponse) {
	return aggregateTestLineItemsWithAccounts(result, nil)
}

func aggregateTestLineItemsWithAccounts(result *aiTransactionDataParsedResult, accountMap map[string]*models.Account) ([]*models.RecognizedTransactionResult, []*models.ImportTransactionWarningResponse) {
	parser := &aiTransactionDataParser{}
	warningCollector := converter.NewImportWarningCollector()
	transactions := parser.aggregateReceiptLineItems(core.NewNullContext(), &models.User{Uid: 1}, result, accountMap, warningCollector)

	return transactions, warningCollector.GetWarnings()
}

func createTestAccountMap() map[string]*models.Account {
	return map[string]*models.Account{
		"Bargeld":   {Name: "Bargeld", Category: models.ACCOUNT_CATEGORY_CASH},
		"Girokonto": {Name: "Girokonto", Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT},
		"Visa":      {Name: "Visa", Category: models.ACCOUNT_CATEGORY_CREDIT_CARD},
	}
}

func createSingleLineItemResult(paymentMethod string, accountName string) *aiTransactionDataParsedResult {
	return &aiTransactionDataParsedResult{
		PaymentMethod: paymentMethod,
		AccountName:   accountName,
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
		},
	}
}

func TestAggregateReceiptLineItems(t *testing.T) {
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "32.79",
		Time:         "2026-07-28 20:12:00",
		AccountName:  "Checking account",
		LineItems:    createTestReceiptLineItems(),
	})

	assert.Nil(t, warnings)
	assert.Equal(t, 5, len(transactions))

	// the groups keep the order in which their category was first seen
	assert.Equal(t, "Food", transactions[0].CategoryName)
	assert.Equal(t, "9.55", transactions[0].Amount)
	assert.Equal(t, "Broccoli, Kartoffeln früh, Strauchtomaten, Bio Möhren, Milchcreme Cookies", transactions[0].Description)

	assert.Equal(t, "Fruit & Snack", transactions[1].CategoryName)
	assert.Equal(t, "4.48", transactions[1].Amount)

	// the deposits are added onto the drinks they were charged on and are not named separately
	assert.Equal(t, "Drink", transactions[2].CategoryName)
	assert.Equal(t, "4.58", transactions[2].Amount)
	assert.Equal(t, "Frischer O-Saft o.F., Monster Mango Loco", transactions[2].Description)

	assert.Equal(t, "Houseware", transactions[3].CategoryName)
	assert.Equal(t, "0.20", transactions[3].Amount)

	// duplicated identical lines are both real purchases and must both be counted
	assert.Equal(t, "Electronics", transactions[4].CategoryName)
	assert.Equal(t, "13.98", transactions[4].Amount)
	assert.Equal(t, "Akku Ni-MH-0532273, Akku Ni-MH-0532273", transactions[4].Description)

	for i := 0; i < len(transactions); i++ {
		assert.Equal(t, "expense", transactions[i].Type)
		assert.Equal(t, "2026-07-28 20:12:00", transactions[i].Time)
		assert.Equal(t, "Checking account", transactions[i].AccountName)
	}
}

func TestAggregateReceiptLineItems_TotalMismatchReportsWarning(t *testing.T) {
	lineItems := createTestReceiptLineItems()
	lineItems[2].Price = "4.69" // Strauchtomaten misread, 3.00 too high

	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "32.79",
		LineItems:    lineItems,
	})

	assert.Equal(t, 5, len(transactions))
	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, models.IMPORT_TRANSACTION_WARNING_RECEIPT_TOTAL_MISMATCH, warnings[0].Type)
	assert.Equal(t, 12, warnings[0].LineItemCount) // 14 lines, of which the 2 deposits were merged
	assert.Equal(t, "35.79", warnings[0].CalculatedTotal)
	assert.Equal(t, "32.79", warnings[0].StatedTotal)
	assert.Equal(t, "3.00", warnings[0].Difference)
}

func TestAggregateReceiptLineItems_SmallMismatchIsIgnored(t *testing.T) {
	lineItems := createTestReceiptLineItems()
	lineItems[2].Price = "1.89" // 0.20 too high, below the 0.50 threshold

	_, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "32.79",
		LineItems:    lineItems,
	})

	assert.Nil(t, warnings)
}

func TestAggregateReceiptLineItems_MismatchExactlyAtThreshold(t *testing.T) {
	lineItems := createTestReceiptLineItems()
	lineItems[2].Price = "1.19" // 0.50 too low

	_, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "32.79",
		LineItems:    lineItems,
	})

	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, "-0.50", warnings[0].Difference)
}

func TestAggregateReceiptLineItems_NoReceiptTotalSkipsValidation(t *testing.T) {
	lineItems := createTestReceiptLineItems()
	lineItems[2].Price = "9.99"

	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: lineItems,
	})

	assert.Equal(t, 5, len(transactions))
	assert.Nil(t, warnings)
}

func TestAggregateReceiptLineItems_UnknownCategoryIsNotDropped(t *testing.T) {
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
			{Name: "Irgendwas", Price: "2.50", Category: "Sonstiges"},
		},
	})

	// an unknown category is passed through, the importer creates it as a new category
	assert.Equal(t, 2, len(transactions))
	assert.Equal(t, "Sonstiges", transactions[1].CategoryName)
	assert.Equal(t, "2.50", transactions[1].Amount)
}

func TestAggregateReceiptLineItems_NegativeAndUnparseablePricesAreSkipped(t *testing.T) {
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "1.49",
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
			{Name: "Leergut", Price: "-3.75", Category: "Drink"},
			{Name: "Unlesbar", Price: "keine Ahnung", Category: "Food"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "Food", transactions[0].CategoryName)
	assert.Equal(t, "1.49", transactions[0].Amount)
	assert.Equal(t, "Broccoli", transactions[0].Description)

	// only the kept line item counts towards the receipt total, so it still matches
	assert.Nil(t, warnings)
}

func TestAggregateReceiptLineItems_ZeroAmountGroupIsNotEmitted(t *testing.T) {
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
			{Name: "Gratisbeigabe", Price: "0.00", Category: "Gifts"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "Food", transactions[0].CategoryName)
}

func TestAggregateReceiptLineItems_DecimalPrecisionIsExact(t *testing.T) {
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "A", Price: "0.1", Category: "Food"},
			{Name: "B", Price: "0.2", Category: "Food"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "0.30", transactions[0].Amount)
}

func TestAggregateReceiptLineItems_EmptyLineItems(t *testing.T) {
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{},
	})

	assert.Equal(t, 0, len(transactions))
	assert.Nil(t, warnings)
}

func TestResolveReceiptAccountName_ByPaymentMethod(t *testing.T) {
	accountMap := createTestAccountMap()

	for paymentMethod, expectedAccountName := range map[string]string{
		"cash":   "Bargeld",
		"debit":  "Girokonto",
		"credit": "Visa",
	} {
		transactions, _ := aggregateTestLineItemsWithAccounts(createSingleLineItemResult(paymentMethod, ""), accountMap)

		assert.Equal(t, 1, len(transactions))
		assert.Equal(t, expectedAccountName, transactions[0].AccountName, "payment method %s", paymentMethod)
	}
}

func TestResolveReceiptAccountName_PaymentMethodBeatsRecognizedAccount(t *testing.T) {
	// the payment line is printed evidence, the account name is only the model picking from a list
	transactions, _ := aggregateTestLineItemsWithAccounts(createSingleLineItemResult("cash", "Girokonto"), createTestAccountMap())

	assert.Equal(t, "Bargeld", transactions[0].AccountName)
}

func TestResolveReceiptAccountName_SeveralAccountsInCategory(t *testing.T) {
	accountMap := createTestAccountMap()
	accountMap["Mastercard"] = &models.Account{Name: "Mastercard", Category: models.ACCOUNT_CATEGORY_CREDIT_CARD}

	// the model named one of them, so that one is used
	transactions, _ := aggregateTestLineItemsWithAccounts(createSingleLineItemResult("credit", "Mastercard"), accountMap)
	assert.Equal(t, "Mastercard", transactions[0].AccountName)

	// it named none of them, so the ambiguity is not guessed at
	transactions, _ = aggregateTestLineItemsWithAccounts(createSingleLineItemResult("credit", ""), accountMap)
	assert.Equal(t, "", transactions[0].AccountName)
}

func TestResolveReceiptAccountName_CardFallsBackToTheOtherCardCategory(t *testing.T) {
	// the real-world setup that broke this: one cash account and one card account filed as a
	// checking account, with no credit card account at all
	accountMap := map[string]*models.Account{
		"Wallet": {Name: "Wallet", Category: models.ACCOUNT_CATEGORY_CASH},
		"Card":   {Name: "Card", Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT},
	}

	transactions, _ := aggregateTestLineItemsWithAccounts(createSingleLineItemResult("credit", ""), accountMap)
	assert.Equal(t, "Card", transactions[0].AccountName)

	transactions, _ = aggregateTestLineItemsWithAccounts(createSingleLineItemResult("debit", ""), accountMap)
	assert.Equal(t, "Card", transactions[0].AccountName)

	transactions, _ = aggregateTestLineItemsWithAccounts(createSingleLineItemResult("cash", ""), accountMap)
	assert.Equal(t, "Wallet", transactions[0].AccountName)
}

func TestResolveReceiptAccountName_Fallbacks(t *testing.T) {
	accountMap := createTestAccountMap()

	// no account of that category exists, and cash has no second category to fall back to
	delete(accountMap, "Bargeld")
	transactions, _ := aggregateTestLineItemsWithAccounts(createSingleLineItemResult("cash", "Girokonto"), accountMap)
	assert.Equal(t, "Girokonto", transactions[0].AccountName)

	// payment method not recognized, or absent entirely
	transactions, _ = aggregateTestLineItemsWithAccounts(createSingleLineItemResult("gutschein", "Girokonto"), createTestAccountMap())
	assert.Equal(t, "Girokonto", transactions[0].AccountName)

	transactions, _ = aggregateTestLineItemsWithAccounts(createSingleLineItemResult("", "Girokonto"), createTestAccountMap())
	assert.Equal(t, "Girokonto", transactions[0].AccountName)

	// no accounts at all
	transactions, _ = aggregateTestLineItemsWithAccounts(createSingleLineItemResult("cash", "Girokonto"), nil)
	assert.Equal(t, "Girokonto", transactions[0].AccountName)
}

func TestResolveReceiptAccountName_IsCaseInsensitive(t *testing.T) {
	transactions, _ := aggregateTestLineItemsWithAccounts(createSingleLineItemResult("  Cash  ", ""), createTestAccountMap())

	assert.Equal(t, "Bargeld", transactions[0].AccountName)
}

func TestAggregateReceiptLineItems_DepositIsMergedIntoTheItemAboveIt(t *testing.T) {
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Frischer O-Saft o.F.", Price: "2.59", Category: "Drink"},
			{Name: "Pfand 0,25 EM", Price: "0.25", Deposit: true, Category: "Drink"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "2.84", transactions[0].Amount)
	assert.Equal(t, "Frischer O-Saft o.F.", transactions[0].Description)
}

func TestAggregateReceiptLineItems_DepositIsDetectedWithoutTheFlag(t *testing.T) {
	// the model forgot the flag, but the money must still land on the drink and the deposit
	// must still stay out of the description
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Frischer O-Saft o.F.", Price: "2.59", Category: "Drink"},
			{Name: "PFAND EINWEG", Price: "0.25", Category: "Drink"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "2.84", transactions[0].Amount)
	assert.Equal(t, "Frischer O-Saft o.F.", transactions[0].Description)
}

func TestAggregateReceiptLineItems_DepositInheritsTheCategoryOfTheItemAboveIt(t *testing.T) {
	// the model filed the deposit under the wrong category; merging repairs that
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Frischer O-Saft o.F.", Price: "2.59", Category: "Drink"},
			{Name: "Pfand 0,25 EM", Price: "0.25", Deposit: true, Category: "Food"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "Drink", transactions[0].CategoryName)
	assert.Equal(t, "2.84", transactions[0].Amount)
}

func TestAggregateReceiptLineItems_LeadingDepositIsKeptSoNoMoneyIsLost(t *testing.T) {
	// nothing precedes it, so it cannot be merged - it stays a line of its own rather than vanishing
	transactions, _ := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Pfand 0,25 EM", Price: "0.25", Deposit: true, Category: "Drink"},
			{Name: "Frischer O-Saft o.F.", Price: "2.59", Category: "Drink"},
		},
	})

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "2.84", transactions[0].Amount)
	assert.Equal(t, "Pfand 0,25 EM, Frischer O-Saft o.F.", transactions[0].Description)
}

func TestParseReceiptAmount(t *testing.T) {
	for amount, expectedValue := range map[string]int64{
		"32.79":     3279, // what the prompt asks for
		"32,79":     3279, // what a German receipt prints, and what the model actually returned
		"0,25":      25,
		"1.234,56":  123456, // German thousands separator
		"1,234.56":  123456, // English thousands separator
		"1234.5":    123450,
		"-1,50":     -150,
		"32,79 EUR": 3279,
		"€ 32,79":   3279,
		"  32,79  ": 3279,
	} {
		actualValue, err := parseReceiptAmount(amount)

		assert.Nil(t, err, "amount %s", amount)
		assert.Equal(t, expectedValue, actualValue, "amount %s", amount)
	}

	_, err := parseReceiptAmount("keine Ahnung")
	assert.NotNil(t, err)
}

func TestAggregateReceiptLineItems_GermanDecimalCommas(t *testing.T) {
	// the model echoed the receipt verbatim instead of converting; nothing may be dropped
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "32,79",
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Frischer O-Saft o.F.", Price: "2,59", Category: "Drink"},
			{Name: "Pfand 0,25 EM", Price: "0,25", Category: "Drink"},
			{Name: "Milchcreme Cookies", Price: "1,49", Category: "Food"},
		},
	})

	assert.Equal(t, 2, len(transactions))
	assert.Equal(t, "2.84", transactions[0].Amount)
	assert.Equal(t, "1.49", transactions[1].Amount)

	// 4.33 against a stated 32.79, so the mismatch is now detected instead of being skipped
	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, "4.33", warnings[0].CalculatedTotal)
	assert.Equal(t, "32.79", warnings[0].StatedTotal)
	assert.Equal(t, "-28.46", warnings[0].Difference)
}

func TestTruncateTransactionDescription(t *testing.T) {
	assert.Equal(t, "Broccoli, Kiwi", truncateTransactionDescription("Broccoli, Kiwi"))

	longDescription := strings.TrimSuffix(strings.Repeat("Kartoffeln früh, ", 30), ", ")
	truncated := truncateTransactionDescription(longDescription)

	assert.LessOrEqual(t, len([]rune(truncated)), maxTransactionDescriptionLength)
	assert.True(t, strings.HasSuffix(truncated, "…"))
	// the cut lands on an item boundary, so no item name is left half written
	assert.True(t, strings.HasSuffix(truncated, "Kartoffeln früh…"))
}
