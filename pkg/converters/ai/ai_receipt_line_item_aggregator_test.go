package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
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
	transactions, warnings, _ := aggregateTestLineItemsWithReceipt(result, accountMap)
	return transactions, warnings
}

func aggregateTestLineItemsWithReceipt(result *aiTransactionDataParsedResult, accountMap map[string]*models.Account) ([]*models.RecognizedTransactionResult, []*models.ImportTransactionWarningResponse, *models.ImportReceiptResponse) {
	parser := &aiTransactionDataParser{}
	warningCollector := converter.NewImportWarningCollector()
	receiptCollector := converter.NewImportReceiptCollector()
	transactions := parser.aggregateReceiptLineItems(core.NewNullContext(), &models.User{Uid: 1}, result, accountMap, warningCollector, receiptCollector)

	return transactions, warningCollector.GetWarnings(), receiptCollector.GetReceipt()
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

func TestAggregateReceiptLineItems_ReportsTheLinesEachTransactionWasSummedFrom(t *testing.T) {
	transactions, _, receipt := aggregateTestLineItemsWithReceipt(&aiTransactionDataParsedResult{
		ReceiptTotal: "32.79",
		Time:         "2026-07-28 20:12:00",
		AccountName:  "Checking account",
		LineItems:    createTestReceiptLineItems(),
	}, nil)

	assert.NotNil(t, receipt)
	assert.Equal(t, 12, len(receipt.LineItems)) // 14 printed lines, of which the 2 deposits were merged

	// the lines keep the order they are printed in, so that the user can follow them down the receipt
	assert.Equal(t, "Broccoli", receipt.LineItems[0].Name)
	assert.Equal(t, int64(149), receipt.LineItems[0].Amount)
	assert.Equal(t, "Food", receipt.LineItems[0].CategoryName)

	// a deposit is not a line of its own, it is part of what the drink above it cost
	assert.Equal(t, "Frischer O-Saft o.F.", receipt.LineItems[6].Name)
	assert.Equal(t, int64(284), receipt.LineItems[6].Amount)
	assert.Equal(t, "Drink", receipt.LineItems[6].CategoryName)

	// every category's lines add up to exactly the transaction that category produced
	amountsByCategoryName := make(map[string]int64, len(transactions))

	for _, lineItem := range receipt.LineItems {
		amountsByCategoryName[lineItem.CategoryName] += lineItem.Amount
	}

	assert.Equal(t, len(transactions), len(amountsByCategoryName))

	for _, transaction := range transactions {
		assert.Equal(t, transaction.Amount, utils.FormatAmount(amountsByCategoryName[transaction.CategoryName]))
	}
}

func TestAggregateReceiptLineItems_ReportsNoReceiptWhenNoLineCouldBeParsed(t *testing.T) {
	_, _, receipt := aggregateTestLineItemsWithReceipt(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "unreadable", Category: "Food"},
		},
	}, nil)

	assert.Nil(t, receipt)
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

func TestAggregateReceiptLineItems_UnparseablePriceIsSkippedAndOversizedCreditIsNotImported(t *testing.T) {
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "1.49",
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
			{Name: "Leergut", Price: "-3.75", Category: "Drink"},
			{Name: "Unlesbar", Price: "keine Ahnung", Category: "Food"},
		},
	})

	// the credit is larger than the item above it, so it is a refund for something else rather than a
	// discount on the broccoli - subtracting it there would turn a real purchase negative and lose both
	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "Food", transactions[0].CategoryName)
	assert.Equal(t, "1.49", transactions[0].Amount)
	assert.Equal(t, "Broccoli", transactions[0].Description)

	// the credit still counts towards what the receipt totals to, so the unreadable line shows up as a
	// gap instead of silently disappearing
	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, models.IMPORT_TRANSACTION_WARNING_RECEIPT_TOTAL_MISMATCH, warnings[0].Type)
	assert.Equal(t, "-2.26", warnings[0].CalculatedTotal)
	assert.Equal(t, "-3.75", warnings[0].Difference)
}

func TestAggregateReceiptLineItems_DiscountReducesTheItemItWasPrintedUnder(t *testing.T) {
	// a Lidl "Preisvorteil" is printed as its own negative line under the item it applies to. Skipping
	// it would overstate that category and leave the sum permanently above the printed total.
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		ReceiptTotal: "4.38",
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Bioland Strauchtoma.", Price: "2.59", Category: "Food"},
			{Name: "Möhren", Price: "1.99", Category: "Vegetables"},
			{Name: "Preisvorteil", Price: "-0.20", Category: "Vegetables"},
		},
	})

	assert.Equal(t, 2, len(transactions))
	assert.Equal(t, "Food", transactions[0].CategoryName)
	assert.Equal(t, "2.59", transactions[0].Amount)

	assert.Equal(t, "Vegetables", transactions[1].CategoryName)
	assert.Equal(t, "1.79", transactions[1].Amount)

	// the discount is not a purchase, so it never shows up in the description
	assert.Equal(t, "Möhren", transactions[1].Description)

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

// the failure this whole two stage prompt exists to catch, taken from a real Lidl receipt: the model
// transcribed all 18 printed lines and then itemized only 16 of them, so 2.40 EUR left the import
// without a word. The line item sum still looks plausible on its own - only the transcript proves
// which lines went missing.
func TestAggregateReceiptLineItems_TranscribedButUnitemizedLinesAreReported(t *testing.T) {
	_, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		RawLines: []string{
			"Toastbrötchen Weizen        0,99 A",
			"Papiertragetasc  0,20 x 2   0,40 B",
			"Biokompost Papierb.         1,49 B",
			"Haushaltshel-0492476        2,00 B",
		},
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Toastbrötchen Weizen", Price: "0.99", Category: "Food"},
			{Name: "Biokompost Papierb.", Price: "1.49", Category: "Houseware"},
		},
	})

	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, models.IMPORT_TRANSACTION_WARNING_RECEIPT_LINES_NOT_ITEMIZED, warnings[0].Type)
	assert.Equal(t, 2, warnings[0].LineItemCount)
	assert.Equal(t, []string{
		"Papiertragetasc  0,20 x 2   0,40 B",
		"Haushaltshel-0492476        2,00 B",
	}, warnings[0].MissingLines)
}

// without a transcript there is nothing to compare the items against, so the check cannot run. It must
// not invent a warning out of the missing list either - a model that skipped stage 1 has not told us
// that anything was lost, only that it cannot be checked, and that distinction lives in the log.
func TestAggregateReceiptLineItems_MissingTranscriptRaisesNoWarning(t *testing.T) {
	transactions, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
			{Name: "Kiwi", Price: "2.29", Category: "Fruit & Snack"},
		},
	})

	assert.Equal(t, 2, len(transactions))
	assert.Nil(t, warnings)
}

func TestAggregateReceiptLineItems_FullyItemizedTranscriptIsNotReported(t *testing.T) {
	_, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		RawLines: []string{
			"Broccoli               1,49 A",
			"Frischer O-Saft o.F.   2,59 B",
			"Pfand 0,25 EM          0,25 B",
		},
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Broccoli", Price: "1.49", Category: "Food"},
			{Name: "Frischer O-Saft o.F.", Price: "2.59", Category: "Drink"},
			{Name: "Pfand 0,25 EM", Price: "0.25", Deposit: true, Category: "Drink"},
		},
	})

	assert.Nil(t, warnings)
}

// the model does not always copy a name across the two stages character for character. The count is
// still trustworthy and worth reporting, but naming lines from a pairing that did not line up would
// accuse the wrong ones.
func TestAggregateReceiptLineItems_UnreliablePairingReportsCountWithoutNamingLines(t *testing.T) {
	_, warnings := aggregateTestLineItems(&aiTransactionDataParsedResult{
		RawLines: []string{
			"Broccoli               1,49 A",
			"Kiwi                   2,29 A",
			"Akku Ni-MH             6,99 B",
		},
		LineItems: []*models.RecognizedReceiptLineItem{
			{Name: "Brokkoli", Price: "1.49", Category: "Food"},
		},
	})

	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, models.IMPORT_TRANSACTION_WARNING_RECEIPT_LINES_NOT_ITEMIZED, warnings[0].Type)
	assert.Equal(t, 2, warnings[0].LineItemCount)
	assert.Nil(t, warnings[0].MissingLines)
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
