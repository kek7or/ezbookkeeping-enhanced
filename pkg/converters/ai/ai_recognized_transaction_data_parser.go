package ai

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/llm"
	"github.com/mayswind/ezbookkeeping/pkg/llm/data"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/templates"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// aiTransactionDataParser defines the interface for parsing transaction data using AI
type aiTransactionDataParser struct {
	currentConfig *settings.Config
}

// aiTransactionDataParsedResult defines the structure of parsed transaction data result
type aiTransactionDataParsedResult struct {
	Transactions []*models.RecognizedTransactionResult `json:"transactions"`
	LineItems    []*models.RecognizedReceiptLineItem   `json:"line_items"`
	// the verbatim transcription of the item block the model writes before it builds the line
	// items; it is a scratchpad and is not imported, but comparing the two counts shows whether
	// a printed line was lost between reading the receipt and structuring it
	RawLines      []string `json:"raw_lines"`
	ReceiptTotal  string   `json:"receipt_total"`
	Merchant      string   `json:"merchant"`
	Time          string   `json:"time"`
	AccountName   string   `json:"account"`
	PaymentMethod string   `json:"payment_method"`
}

// parseText processes the input text data and returns the recognized transaction results using AI
func (p *aiTransactionDataParser) parseText(c core.Context, user *models.User, fileData string, additionalPrompt string, defaultTimezone *time.Location, accountMap map[string]*models.Account, expenseCategoryMap map[string]map[string]*models.TransactionCategory, incomeCategoryMap map[string]map[string]*models.TransactionCategory, transferCategoryMap map[string]map[string]*models.TransactionCategory, tagMap map[string]*models.TransactionTag) ([]*models.RecognizedTransactionResult, error) {
	if p.currentConfig == nil || p.currentConfig.TextRecognitionLLMConfig == nil || p.currentConfig.TextRecognitionLLMConfig.LLMProvider == "" || !p.currentConfig.TransactionFromAITextRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	text := strings.TrimSpace(fileData)

	if len(text) == 0 {
		log.Warnf(c, "[ai_recognized_transaction_data_parser.parseText] input text is empty for user \"uid:%d\"", user.Uid)
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	systemPrompt, err := p.buildRecognitionSystemPrompt(c, user, templates.SYSTEM_PROMPT_BATCH_TRANSACTION_TEXT_RECOGNITION, additionalPrompt, defaultTimezone, accountMap, expenseCategoryMap, incomeCategoryMap, transferCategoryMap, tagMap)

	if err != nil {
		return nil, err
	}

	llmRequest := &data.LargeLanguageModelRequest{
		Stream:         false,
		SystemPrompt:   systemPrompt,
		UserPrompt:     []byte(text),
		UserPromptType: data.LARGE_LANGUAGE_MODEL_REQUEST_PROMPT_TYPE_TEXT,
	}

	llmResponse, err := llm.Container.GetJsonResponseByTextRecognitionModel(c, user.Uid, p.currentConfig, llmRequest)

	if err != nil {
		log.Errorf(c, "[ai_recognized_transaction_data_parser.parseText] failed to get llm response for user \"uid:%d\", because %s", user.Uid, err.Error())
		return nil, errs.ErrOperationFailed
	}

	result, err := p.parseRecognizedResult(c, user, llmResponse)

	if err != nil {
		return nil, err
	}

	if len(result.Transactions) < 1 {
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	return result.Transactions, nil
}

// parseText processes the input image data and returns the recognized transaction results using AI
func (p *aiTransactionDataParser) parseImage(c core.Context, user *models.User, imageData []byte, additionalPrompt string, additionalOptions converter.TransactionDataImporterOptions, defaultTimezone *time.Location, accountMap map[string]*models.Account, expenseCategoryMap map[string]map[string]*models.TransactionCategory, incomeCategoryMap map[string]map[string]*models.TransactionCategory, transferCategoryMap map[string]map[string]*models.TransactionCategory, tagMap map[string]*models.TransactionTag) ([]*models.RecognizedTransactionResult, error) {
	if p.currentConfig == nil || p.currentConfig.ReceiptImageRecognitionLLMConfig == nil || p.currentConfig.ReceiptImageRecognitionLLMConfig.LLMProvider == "" || !p.currentConfig.TransactionFromAIImageRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	if len(imageData) == 0 {
		log.Warnf(c, "[ai_recognized_transaction_data_parser.parseImage] input image is empty for user \"uid:%d\"", user.Uid)
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	systemPrompt, err := p.buildRecognitionSystemPrompt(c, user, templates.SYSTEM_PROMPT_BATCH_RECEIPT_IMAGE_RECOGNITION, additionalPrompt, defaultTimezone, accountMap, expenseCategoryMap, incomeCategoryMap, transferCategoryMap, tagMap)

	if err != nil {
		return nil, err
	}

	llmRequest := &data.LargeLanguageModelRequest{
		Stream:                false,
		SystemPrompt:          systemPrompt,
		UserPrompt:            imageData,
		UserPromptType:        data.LARGE_LANGUAGE_MODEL_REQUEST_PROMPT_TYPE_IMAGE_URL,
		UserPromptContentType: additionalOptions.GetAIImageContentType(),
	}

	llmResponse, err := llm.Container.GetJsonResponseByReceiptImageRecognitionModel(c, user.Uid, p.currentConfig, llmRequest)

	if err != nil {
		log.Errorf(c, "[ai_recognized_transaction_data_parser.parseImage] failed to get llm response for user \"uid:%d\", because %s", user.Uid, err.Error())
		return nil, errs.ErrOperationFailed
	}

	result, err := p.parseRecognizedResult(c, user, llmResponse)

	if err != nil {
		return nil, err
	}

	// a receipt listing several purchased lines is returned as line items which are grouped and summed here,
	// any other image (a single voucher, a transfer confirmation) is returned as transactions directly
	if len(result.LineItems) > 0 {
		transactions := p.aggregateReceiptLineItems(c, user, result, accountMap, additionalOptions.GetWarningCollector(), additionalOptions.GetReceiptCollector(), additionalOptions.GetReceiptLineItemCategories())

		if len(transactions) > 0 {
			return transactions, nil
		}
	}

	if len(result.Transactions) < 1 {
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	p.checkReceiptWasItemized(c, user, result, additionalOptions.GetWarningCollector())

	return result.Transactions, nil
}

// checkReceiptWasItemized reports an image the model read as a receipt and then answered with a
// single transaction instead of with the articles printed on it.
//
// Nothing can be recovered when that happens: the price of each article was never returned, so
// there is no itemization to correct and no distribution of categories to review. What the warning
// is for is that the user must not be left believing there was one - an unremarked transaction
// summing a whole receipt looks exactly like a transaction that was checked.
//
// It is raised only when the answer still carries receipt-level facts: the shop, the printed total,
// or the transcript of the item block. An image that genuinely is not a receipt - a voucher, a
// transfer confirmation - carries none of those and is answered with transactions quite correctly,
// so this stays silent for it rather than crying wolf.
func (p *aiTransactionDataParser) checkReceiptWasItemized(c core.Context, user *models.User, result *aiTransactionDataParsedResult, warningCollector *converter.ImportWarningCollector) {
	if warningCollector == nil {
		return
	}

	if strings.TrimSpace(result.Merchant) == "" && strings.TrimSpace(result.ReceiptTotal) == "" && len(result.RawLines) < 1 {
		return
	}

	log.Warnf(c, "[ai_recognized_transaction_data_parser.checkReceiptWasItemized] model answered a receipt with %d transactions instead of its line items for user \"uid:%d\", %d lines had been transcribed", len(result.Transactions), user.Uid, len(result.RawLines))

	warningCollector.Add(&models.ImportTransactionWarningResponse{
		Type:          models.IMPORT_TRANSACTION_WARNING_RECEIPT_NOT_ITEMIZED,
		LineItemCount: len(result.RawLines),
	})
}

// buildRecognitionSystemPrompt returns the system prompt for AI recognition based on the provided template and user data
func (p *aiTransactionDataParser) buildRecognitionSystemPrompt(c core.Context, user *models.User, templateName templates.KnownTemplate, additionalPrompt string, defaultTimezone *time.Location, accountMap map[string]*models.Account, expenseCategoryMap, incomeCategoryMap, transferCategoryMap map[string]map[string]*models.TransactionCategory, tagMap map[string]*models.TransactionTag) (string, error) {
	accountNames := p.getAccountLines(accountMap)
	expenseCategoryNames := p.getCategoryLines(c, user, expenseCategoryMap)
	incomeCategoryNames := p.getCategoryLines(c, user, incomeCategoryMap)
	transferCategoryNames := p.getCategoryLines(c, user, transferCategoryMap)
	tagNames := p.getTagNames(tagMap)

	systemPrompt, err := templates.GetTextTemplate(templateName)

	if err != nil {
		log.Errorf(c, "[ai_recognized_transaction_data_parser.buildRecognitionSystemPrompt] failed to get prompt template for user \"uid:%d\", because %s", user.Uid, err.Error())
		return "", errs.ErrOperationFailed
	}

	systemPromptParams := map[string]any{
		"CurrentDateTime":          utils.FormatUnixTimeToLongDateTime(time.Now().Unix(), defaultTimezone),
		"AllExpenseCategoryNames":  strings.Join(expenseCategoryNames, "\n"),
		"AllIncomeCategoryNames":   strings.Join(incomeCategoryNames, "\n"),
		"AllTransferCategoryNames": strings.Join(transferCategoryNames, "\n"),
		"AllAccountNames":          strings.Join(accountNames, "\n"),
		"AllTagNames":              strings.Join(tagNames, "\n"),
		"AdditionalNotes":          additionalPrompt,
	}

	var bodyBuffer bytes.Buffer

	if err := systemPrompt.Execute(&bodyBuffer, systemPromptParams); err != nil {
		log.Errorf(c, "[ai_recognized_transaction_data_parser.buildRecognitionSystemPrompt] failed to render prompt template for user \"uid:%d\", because %s", user.Uid, err.Error())
		return "", errs.ErrOperationFailed
	}

	return strings.ReplaceAll(bodyBuffer.String(), "\r\n", "\n"), nil
}

func (p *aiTransactionDataParser) parseRecognizedResult(c core.Context, user *models.User, llmResponse *data.LargeLanguageModelTextualResponse) (*aiTransactionDataParsedResult, error) {
	if llmResponse == nil || len(llmResponse.Content) == 0 || strings.HasPrefix(llmResponse.Content, "[]") {
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	var result *aiTransactionDataParsedResult

	if err := json.Unmarshal([]byte(llmResponse.Content), &result); err != nil {
		log.Errorf(c, "[ai_recognized_transaction_data_parser.parseRecognizedResult] failed to unmarshal batch llm response \"%s\" for user \"uid:%d\", because %s", llmResponse.Content, user.Uid, err.Error())
		return nil, errs.ErrOperationFailed
	}

	if result == nil {
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	repairMojibakeInParsedResult(c, user, result)

	return result, nil
}

// repairMojibakeInParsedResult fixes text the model returned with its non-ASCII
// characters corrupted, writing "HÃ¤hnchen-Brustfilet" where the receipt printed
// "Hähnchen-Brustfilet".
//
// The corruption is the model's own: the raw response body already contains it,
// so there is nothing upstream to correct. It shows up reliably on receipts in
// languages with accented characters, which makes every affected item name and
// category wrong in the imported transaction.
func repairMojibakeInParsedResult(c core.Context, user *models.User, result *aiTransactionDataParsedResult) {
	repaired := false

	for i := 0; i < len(result.LineItems); i++ {
		lineItem := result.LineItems[i]

		if lineItem == nil {
			continue
		}

		repaired = utils.RepairMojibakeInAllFields(&lineItem.Name, &lineItem.Category, &lineItem.Reason) || repaired
	}

	for i := 0; i < len(result.Transactions); i++ {
		transaction := result.Transactions[i]

		if transaction == nil {
			continue
		}

		repaired = utils.RepairMojibakeInAllFields(
			&transaction.Description,
			&transaction.CategoryName,
			&transaction.AccountName,
			&transaction.DestinationAccountName,
		) || repaired

		utils.RepairMojibakeSlice(transaction.TagNames)
	}

	repaired = utils.RepairMojibakeInAllFields(&result.AccountName) || repaired
	utils.RepairMojibakeSlice(result.RawLines)

	if repaired {
		// Logged rather than fixed silently: a model that keeps doing this is
		// one worth replacing, and that is invisible if the repair hides it.
		log.Warnf(c, "[ai_recognized_transaction_data_parser.repairMojibakeInParsedResult] repaired mojibake in the model response for user \"uid:%d\"", user.Uid)
	}
}

// account category names as they are shown in the user interface, so that the model sees an
// account list it can match a payment method against ("paid by card" -> the Credit Card account)
var aiAccountCategoryNames = map[models.AccountCategory]string{
	models.ACCOUNT_CATEGORY_CASH:                   "Cash",
	models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT:       "Checking Account",
	models.ACCOUNT_CATEGORY_CREDIT_CARD:            "Credit Card",
	models.ACCOUNT_CATEGORY_VIRTUAL:                "Virtual Account",
	models.ACCOUNT_CATEGORY_DEBT:                   "Debt Account",
	models.ACCOUNT_CATEGORY_RECEIVABLES:            "Receivables",
	models.ACCOUNT_CATEGORY_INVESTMENT:             "Investment Account",
	models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT:        "Savings Account",
	models.ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT: "Certificate of Deposit",
}

// getAccountLines returns the account list rendered for the system prompt, annotated with each
// account's category, for example "Wallet (Cash)". Without the category the model has no way to
// tell which account a cash or card payment belongs to, since account names are user chosen.
func (p *aiTransactionDataParser) getAccountLines(accountMap map[string]*models.Account) []string {
	lines := make([]string, 0, len(accountMap))

	for _, account := range accountMap {
		if categoryName, exists := aiAccountCategoryNames[account.Category]; exists {
			lines = append(lines, account.Name+" ("+categoryName+")")
		} else {
			lines = append(lines, account.Name)
		}
	}

	sort.Strings(lines)

	return lines
}

// getCategoryLines returns the category list rendered for the system prompt, grouped by their parent
// category and annotated with the user defined category description, for example:
//
//	Food & Drink:
//	  - Food: everyday groceries
//	  - Drink
//
// Transactions are always assigned to a sub category, so the parent category is shown for context only
// and the sub category name is what the model is asked to return. The output is sorted so that the same
// user data always renders the exact same prompt.
func (p *aiTransactionDataParser) getCategoryLines(c core.Context, user *models.User, categoryMap map[string]map[string]*models.TransactionCategory) []string {
	subCategoriesByParentName := make(map[string][]*models.TransactionCategory)

	for subCategoryName, subCategoryMapByParentName := range categoryMap {
		if len(subCategoryMapByParentName) > 1 {
			parentNames := make([]string, 0, len(subCategoryMapByParentName))

			for parentName := range subCategoryMapByParentName {
				parentNames = append(parentNames, parentName)
			}

			sort.Strings(parentNames)
			log.Warnf(c, "[ai_recognized_transaction_data_parser.getCategoryLines] sub category name \"%s\" exists in multiple parent categories (%s) for user \"uid:%d\", the recognized transactions of this category cannot be resolved to a specific one", subCategoryName, strings.Join(parentNames, ", "), user.Uid)
		}

		for parentName, category := range subCategoryMapByParentName {
			subCategoriesByParentName[parentName] = append(subCategoriesByParentName[parentName], category)
		}
	}

	parentNames := make([]string, 0, len(subCategoriesByParentName))

	for parentName := range subCategoriesByParentName {
		parentNames = append(parentNames, parentName)
	}

	sort.Strings(parentNames)

	lines := make([]string, 0, len(parentNames)+len(categoryMap))

	for _, parentName := range parentNames {
		subCategories := subCategoriesByParentName[parentName]

		sort.SliceStable(subCategories, func(i, j int) bool {
			if subCategories[i].DisplayOrder != subCategories[j].DisplayOrder {
				return subCategories[i].DisplayOrder < subCategories[j].DisplayOrder
			}

			return subCategories[i].Name < subCategories[j].Name
		})

		lines = append(lines, parentName+":")

		for _, subCategory := range subCategories {
			description := strings.TrimSpace(subCategory.Comment)

			if description != "" {
				lines = append(lines, "  - "+subCategory.Name+": "+description)
			} else {
				lines = append(lines, "  - "+subCategory.Name)
			}
		}
	}

	return lines
}

func (p *aiTransactionDataParser) getTagNames(tagMap map[string]*models.TransactionTag) []string {
	names := make([]string, 0, len(tagMap))

	for _, tag := range tagMap {
		names = append(names, tag.Name)
	}

	sort.Strings(names)

	return names
}

func createNewAITextTransactionDataParser(currentConfig *settings.Config) (*aiTransactionDataParser, error) {
	if currentConfig == nil || currentConfig.TextRecognitionLLMConfig == nil || currentConfig.TextRecognitionLLMConfig.LLMProvider == "" || !currentConfig.TransactionFromAITextRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	return &aiTransactionDataParser{
		currentConfig: currentConfig,
	}, nil
}

func createNewAIImageTransactionDataParser(currentConfig *settings.Config) (*aiTransactionDataParser, error) {
	if currentConfig == nil || currentConfig.ReceiptImageRecognitionLLMConfig == nil || currentConfig.ReceiptImageRecognitionLLMConfig.LLMProvider == "" || !currentConfig.TransactionFromAIImageRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	return &aiTransactionDataParser{
		currentConfig: currentConfig,
	}, nil
}
