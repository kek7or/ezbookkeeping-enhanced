package services

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

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

// ReceiptRecognitionService turns a receipt image into a recognized transaction.
//
// This lives in the service layer rather than in an api handler because it has
// two callers with nothing else in common: the synchronous endpoint used by the
// web UI, and the background worker that drains the recognition job queue. Both
// must produce identical results, so there is deliberately only one copy.
type ReceiptRecognitionService struct {
	ServiceUsingConfig
}

// Initialize a receipt recognition service singleton instance
var (
	ReceiptRecognitions = &ReceiptRecognitionService{
		ServiceUsingConfig: ServiceUsingConfig{
			container: settings.Container,
		},
	}
)

// UserEssentialData holds the account, category and tag vocabulary that the model
// is given to choose from, along with the maps used to resolve its answers back
// into ids.
type UserEssentialData struct {
	AccountNames          []string
	AccountMap            map[string]*models.Account
	IncomeCategoryNames   []string
	ExpenseCategoryNames  []string
	TransferCategoryNames []string
	IncomeCategoryMap     map[string]*models.TransactionCategory
	ExpenseCategoryMap    map[string]*models.TransactionCategory
	TransferCategoryMap   map[string]*models.TransactionCategory
	TagNames              []string
	TagMap                map[string]*models.TransactionTag
}

// GetUserEssentialData returns the vocabulary the recognition prompt needs for the specified user
func (s *ReceiptRecognitionService) GetUserEssentialData(c core.Context, uid int64) (*UserEssentialData, error) {
	accounts, err := Accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[receipt_recognitions.GetUserEssentialData] failed to get all accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, err
	}

	result := &UserEssentialData{
		AccountMap:            Accounts.GetVisibleAccountNameMapByList(accounts),
		AccountNames:          make([]string, 0, len(accounts)),
		IncomeCategoryMap:     make(map[string]*models.TransactionCategory),
		IncomeCategoryNames:   make([]string, 0),
		ExpenseCategoryMap:    make(map[string]*models.TransactionCategory),
		ExpenseCategoryNames:  make([]string, 0),
		TransferCategoryMap:   make(map[string]*models.TransactionCategory),
		TransferCategoryNames: make([]string, 0),
	}

	for i := 0; i < len(accounts); i++ {
		if accounts[i].Hidden || accounts[i].Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
			continue
		}

		result.AccountNames = append(result.AccountNames, accounts[i].Name)
	}

	categories, err := TransactionCategories.GetAllCategoriesByUid(c, uid, 0, -1)

	if err != nil {
		log.Errorf(c, "[receipt_recognitions.GetUserEssentialData] failed to get categories for user \"uid:%d\", because %s", uid, err.Error())
		return nil, err
	}

	for i := 0; i < len(categories); i++ {
		category := categories[i]

		if category.Hidden || category.ParentCategoryId == models.LevelOneTransactionCategoryParentId {
			continue
		}

		if category.Type == models.CATEGORY_TYPE_INCOME {
			result.IncomeCategoryMap[category.Name] = category
			result.IncomeCategoryNames = append(result.IncomeCategoryNames, category.Name)
		} else if category.Type == models.CATEGORY_TYPE_EXPENSE {
			result.ExpenseCategoryMap[category.Name] = category
			result.ExpenseCategoryNames = append(result.ExpenseCategoryNames, category.Name)
		} else if category.Type == models.CATEGORY_TYPE_TRANSFER {
			result.TransferCategoryMap[category.Name] = category
			result.TransferCategoryNames = append(result.TransferCategoryNames, category.Name)
		}
	}

	tags, err := TransactionTags.GetAllTagsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[receipt_recognitions.GetUserEssentialData] failed to get tags for user \"uid:%d\", because %s", uid, err.Error())
		return nil, err
	}

	result.TagMap = TransactionTags.GetVisibleTagNameMapByList(tags)
	result.TagNames = make([]string, 0, len(tags))

	for i := 0; i < len(tags); i++ {
		if tags[i].Hidden {
			continue
		}

		result.TagNames = append(result.TagNames, tags[i].Name)
	}

	return result, nil
}

// RecognizeReceiptImage sends the image to the configured model and resolves the
// answer into a transaction. clientTimezone is what relative dates on the receipt
// are interpreted against, so a job processed hours later still resolves the same
// way it would have at submission time.
func (s *ReceiptRecognitionService) RecognizeReceiptImage(c core.Context, uid int64, clientTimezone *time.Location, imageData []byte, contentType string, essentialData *UserEssentialData) (*models.RecognizedTransactionResponse, error) {
	config := s.CurrentConfig()

	if config.ReceiptImageRecognitionLLMConfig == nil || config.ReceiptImageRecognitionLLMConfig.LLMProvider == "" || !config.TransactionFromAIImageRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	systemPrompt, err := templates.GetTextTemplate(templates.SYSTEM_PROMPT_RECEIPT_IMAGE_RECOGNITION)

	if err != nil {
		log.Errorf(c, "[receipt_recognitions.RecognizeReceiptImage] failed to get system prompt template for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	systemPromptParams := map[string]any{
		"CurrentDateTime":          utils.FormatUnixTimeToLongDateTime(time.Now().Unix(), clientTimezone),
		"AllExpenseCategoryNames":  strings.Join(essentialData.ExpenseCategoryNames, "\n"),
		"AllIncomeCategoryNames":   strings.Join(essentialData.IncomeCategoryNames, "\n"),
		"AllTransferCategoryNames": strings.Join(essentialData.TransferCategoryNames, "\n"),
		"AllAccountNames":          strings.Join(essentialData.AccountNames, "\n"),
		"AllTagNames":              strings.Join(essentialData.TagNames, "\n"),
		"AdditionalNotes":          "",
	}

	var bodyBuffer bytes.Buffer
	err = systemPrompt.Execute(&bodyBuffer, systemPromptParams)

	if err != nil {
		log.Errorf(c, "[receipt_recognitions.RecognizeReceiptImage] failed to get final system prompt from template for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	llmRequest := &data.LargeLanguageModelRequest{
		Stream:                false,
		SystemPrompt:          strings.ReplaceAll(bodyBuffer.String(), "\r\n", "\n"),
		UserPrompt:            imageData,
		UserPromptType:        data.LARGE_LANGUAGE_MODEL_REQUEST_PROMPT_TYPE_IMAGE_URL,
		UserPromptContentType: contentType,
	}

	llmResponse, err := llm.Container.GetJsonResponseByReceiptImageRecognitionModel(c, uid, config, llmRequest)

	if err != nil {
		log.Errorf(c, "[receipt_recognitions.RecognizeReceiptImage] failed to get llm response for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if llmResponse == nil || len(llmResponse.Content) == 0 || strings.HasPrefix(llmResponse.Content, "{}") {
		return nil, errs.ErrNoTransactionInformation
	}

	var result *models.RecognizedTransactionResult

	if err := json.Unmarshal([]byte(llmResponse.Content), &result); err != nil {
		log.Errorf(c, "[receipt_recognitions.RecognizeReceiptImage] failed to unmarshal recognized receipt image result from llm response \"%s\" for user \"uid:%d\", because %s", llmResponse.Content, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return s.ParseRecognizedTransactionResult(c, clientTimezone, result, essentialData)
}

// ParseRecognizedTransactionResult resolves the names the model returned back into
// account, category and tag ids. A name the model invented that matches nothing is
// left unset rather than guessed at, so the gap is visible to whoever reviews it.
func (s *ReceiptRecognitionService) ParseRecognizedTransactionResult(c core.Context, clientTimezone *time.Location, recognizedResult *models.RecognizedTransactionResult, essentialData *UserEssentialData) (*models.RecognizedTransactionResponse, error) {
	if recognizedResult == nil {
		return nil, errs.ErrNoTransactionInformation
	}

	recognizedTransactionResponse := &models.RecognizedTransactionResponse{}

	if recognizedResult.Type == "income" {
		recognizedTransactionResponse.Type = models.TRANSACTION_TYPE_INCOME
		s.resolveCategory(recognizedResult.CategoryName, essentialData.IncomeCategoryMap, recognizedTransactionResponse)
	} else if recognizedResult.Type == "expense" {
		recognizedTransactionResponse.Type = models.TRANSACTION_TYPE_EXPENSE
		s.resolveCategory(recognizedResult.CategoryName, essentialData.ExpenseCategoryMap, recognizedTransactionResponse)
	} else if recognizedResult.Type == "transfer" {
		recognizedTransactionResponse.Type = models.TRANSACTION_TYPE_TRANSFER
		s.resolveCategory(recognizedResult.CategoryName, essentialData.TransferCategoryMap, recognizedTransactionResponse)
	} else if len(recognizedResult.Type) == 0 {
		return nil, errs.ErrNoTransactionInformation
	} else {
		log.Errorf(c, "[receipt_recognitions.ParseRecognizedTransactionResult] recognized transaction type \"%s\" is invalid", recognizedResult.Type)
		return nil, errs.ErrOperationFailed
	}

	if len(recognizedResult.Time) > 0 {
		longDateTime := getLongDateTime(recognizedResult.Time)
		timestamp, err := utils.ParseFromLongDateTimeInTimeZone(longDateTime, clientTimezone)

		if err != nil {
			log.Warnf(c, "[receipt_recognitions.ParseRecognizedTransactionResult] recognized time \"%s\" is invalid", recognizedResult.Time)
		} else {
			recognizedTransactionResponse.Time = timestamp.Unix()
		}
	}

	if len(recognizedResult.Amount) > 0 {
		amount, err := utils.ParseAmount(recognizedResult.Amount)

		if err != nil {
			log.Errorf(c, "[receipt_recognitions.ParseRecognizedTransactionResult] recognized amount \"%s\" is invalid", recognizedResult.Amount)
			return nil, errs.ErrOperationFailed
		}

		recognizedTransactionResponse.SourceAmount = amount

		if recognizedTransactionResponse.Type == models.TRANSACTION_TYPE_TRANSFER && len(recognizedResult.DestinationAmount) > 0 {
			destinationAmount, err := utils.ParseAmount(recognizedResult.DestinationAmount)

			if err != nil {
				log.Errorf(c, "[receipt_recognitions.ParseRecognizedTransactionResult] recognized destination amount \"%s\" is invalid", recognizedResult.DestinationAmount)
				return nil, errs.ErrOperationFailed
			}

			recognizedTransactionResponse.DestinationAmount = destinationAmount
		}
	}

	if len(recognizedResult.AccountName) > 0 {
		account, exists := essentialData.AccountMap[recognizedResult.AccountName]

		if exists {
			recognizedTransactionResponse.SourceAccountId = account.AccountId
		}
	}

	if len(recognizedResult.DestinationAccountName) > 0 {
		account, exists := essentialData.AccountMap[recognizedResult.DestinationAccountName]

		if exists {
			recognizedTransactionResponse.DestinationAccountId = account.AccountId
		}
	}

	if len(recognizedResult.TagNames) > 0 {
		tagIds := make([]string, 0, len(recognizedResult.TagNames))

		for i := 0; i < len(recognizedResult.TagNames); i++ {
			tag, exists := essentialData.TagMap[recognizedResult.TagNames[i]]

			if exists {
				tagIds = append(tagIds, utils.Int64ToString(tag.TagId))
			}
		}

		recognizedTransactionResponse.TagIds = tagIds
	}

	if len(recognizedResult.Description) > 0 {
		recognizedTransactionResponse.Comment = recognizedResult.Description
	}

	return recognizedTransactionResponse, nil
}

func (s *ReceiptRecognitionService) resolveCategory(categoryName string, categoryMap map[string]*models.TransactionCategory, response *models.RecognizedTransactionResponse) {
	if len(categoryName) == 0 {
		return
	}

	category, exists := categoryMap[categoryName]

	if exists {
		response.CategoryId = category.CategoryId
	}
}

func getLongDateTime(dateTime string) string {
	if utils.IsValidLongDateTimeFormat(dateTime) {
		return dateTime
	}

	if utils.IsValidLongDateTimeWithoutSecondFormat(dateTime) {
		return dateTime + ":00"
	}

	if utils.IsValidLongDateFormat(dateTime) {
		return dateTime + " 00:00:00"
	}

	return dateTime
}
