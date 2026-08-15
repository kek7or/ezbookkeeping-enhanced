package ollama

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/llm/data"
	"github.com/mayswind/ezbookkeeping/pkg/llm/provider"
	"github.com/mayswind/ezbookkeeping/pkg/llm/provider/common"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

const ollamaChatCompletionsPath = "api/chat"

// OllamaLargeLanguageModelAdapter defines the structure of Ollama large language model adapter
type OllamaLargeLanguageModelAdapter struct {
	common.HttpLargeLanguageModelAdapter
	OllamaServerURL     string
	OllamaModelID       string
	OllamaThinkingLevel settings.LLMThinkingLevel
	OllamaNumCtx        uint32
	OllamaNumPredict    uint32
}

// Sampling options sent with every Ollama request. Everything this application asks a model for is
// transcription of something already printed on a receipt or written in a text - there is exactly
// one right answer and nothing to be creative about, so the sampler is pinned to greedy decoding.
//
// Without these, Ollama falls back to whatever the Modelfile carries, and the stock defaults
// (temperature 0.8, top_p 0.9, top_k 40) pick a fresh token where the receipt only has one. Over a
// twenty line receipt that randomness compounds into skipped lines and prices read off the
// neighbouring row, while the JSON stays well formed enough that nothing downstream notices.
const (
	ollamaRecognitionTemperature = float32(0)
	ollamaRecognitionTopP        = float32(1)
	ollamaRecognitionTopK        = int32(1)

	// a receipt legitimately repeats itself - the same article bought twice, the same price on two
	// lines, "deposit": false on every entry - so the default 1.1 repetition penalty actively pushes
	// the model away from transcribing it correctly. 1.0 disables the penalty.
	ollamaRecognitionRepeatPenalty = float32(1)
)

// OllamaMessageRole defines the role of Ollama chat message
type OllamaMessageRole string

const (
	OllamaMessageRoleSystem OllamaMessageRole = "system"
	OllamaMessageRoleUser   OllamaMessageRole = "user"
)

// OllamaChatRequest defines the structure of Ollama chat request
type OllamaChatRequest struct {
	Model    string                      `json:"model"`
	Stream   bool                        `json:"stream"`
	Messages []*OllamaChatRequestMessage `json:"messages"`
	Think    any                         `json:"think,omitempty"`
	Format   string                      `json:"format,omitempty"`
	Options  *OllamaChatRequestOptions   `json:"options,omitempty"`
}

// OllamaChatRequestOptions defines the runtime options of Ollama chat request. These override the
// parameters baked into the model's Modelfile, so that a correct recognition does not depend on how
// the model was built locally.
type OllamaChatRequestOptions struct {
	Temperature   float32 `json:"temperature"`
	TopP          float32 `json:"top_p"`
	TopK          int32   `json:"top_k"`
	RepeatPenalty float32 `json:"repeat_penalty"`

	// context and output budget are left to the model unless configured, because they cost VRAM and
	// the right value depends on the machine ezBookkeeping talks to rather than on the task
	NumCtx     uint32 `json:"num_ctx,omitempty"`
	NumPredict uint32 `json:"num_predict,omitempty"`
}

// OllamaChatRequestMessage defines the structure of Ollama chat request message
type OllamaChatRequestMessage struct {
	Role    OllamaMessageRole `json:"role"`
	Content string            `json:"content"`
	Images  []string          `json:"images,omitempty"`
}

// OllamaChatResponse defines the structure of Ollama chat response
type OllamaChatResponse struct {
	Message *OllamaChatResponseMessage `json:"message"`
}

// OllamaChatResponseMessage defines the structure of Ollama chat response message
type OllamaChatResponseMessage struct {
	Content *string `json:"content"`
}

// Ollama Chat Thinking Types Mapping
var ollamaChatThinkingTypesMapping = map[settings.LLMThinkingLevel]any{
	settings.LLMThinkingDisabled: false,
	settings.LLMThinkingEnabled:  true,
	settings.LLMThinkingLow:      "low",
	settings.LLMThinkingMedium:   "medium",
	settings.LLMThinkingHigh:     "high",
	settings.LLMThinkingXHigh:    "max",
}

// BuildTextualRequest returns the http request by Ollama large language model adapter
func (p *OllamaLargeLanguageModelAdapter) BuildTextualRequest(c core.Context, uid int64, request *data.LargeLanguageModelRequest, responseType data.LargeLanguageModelResponseFormat) (*http.Request, error) {
	requestBody, err := p.buildJsonRequestBody(c, uid, request, responseType)

	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequest("POST", p.getOllamaRequestUrl(), bytes.NewReader(requestBody))

	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	return httpRequest, nil
}

// ParseTextualResponse returns the textual response by Ollama large language model adapter
func (p *OllamaLargeLanguageModelAdapter) ParseTextualResponse(c core.Context, uid int64, body []byte, responseType data.LargeLanguageModelResponseFormat) (*data.LargeLanguageModelTextualResponse, error) {
	chatResponse := &OllamaChatResponse{}
	err := json.Unmarshal(body, &chatResponse)

	if err != nil {
		log.Errorf(c, "[ollama_large_language_model_adapter.ParseTextualResponse] failed to parse chat response for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	if chatResponse == nil || chatResponse.Message == nil || chatResponse.Message.Content == nil {
		log.Errorf(c, "[ollama_large_language_model_adapter.ParseTextualResponse] chat response is invalid for user \"uid:%d\"", uid)
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	textualResponse := &data.LargeLanguageModelTextualResponse{
		Content: *chatResponse.Message.Content,
	}

	return textualResponse, nil
}

func (p *OllamaLargeLanguageModelAdapter) buildJsonRequestBody(c core.Context, uid int64, request *data.LargeLanguageModelRequest, responseType data.LargeLanguageModelResponseFormat) ([]byte, error) {
	if p.OllamaModelID == "" {
		return nil, errs.ErrInvalidLLMModelId
	}

	chatRequest := &OllamaChatRequest{
		Model:    p.OllamaModelID,
		Stream:   request.Stream,
		Messages: make([]*OllamaChatRequestMessage, 0, 2),
		Options: &OllamaChatRequestOptions{
			Temperature:   ollamaRecognitionTemperature,
			TopP:          ollamaRecognitionTopP,
			TopK:          ollamaRecognitionTopK,
			RepeatPenalty: ollamaRecognitionRepeatPenalty,
			NumCtx:        p.OllamaNumCtx,
			NumPredict:    p.OllamaNumPredict,
		},
	}

	if thinkingLevel, exists := ollamaChatThinkingTypesMapping[p.OllamaThinkingLevel]; exists {
		chatRequest.Think = thinkingLevel
	}

	if request.SystemPrompt != "" {
		chatRequest.Messages = append(chatRequest.Messages, &OllamaChatRequestMessage{
			Role:    OllamaMessageRoleSystem,
			Content: request.SystemPrompt,
		})
	}

	if len(request.UserPrompt) > 0 {
		if request.UserPromptType == data.LARGE_LANGUAGE_MODEL_REQUEST_PROMPT_TYPE_IMAGE_URL {
			imageBase64Data := base64.StdEncoding.EncodeToString(request.UserPrompt)
			chatRequest.Messages = append(chatRequest.Messages, &OllamaChatRequestMessage{
				Role:   OllamaMessageRoleUser,
				Images: []string{imageBase64Data},
			})
		} else {
			chatRequest.Messages = append(chatRequest.Messages, &OllamaChatRequestMessage{
				Role:    OllamaMessageRoleUser,
				Content: string(request.UserPrompt),
			})
		}
	}

	if responseType == data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON {
		chatRequest.Format = "json"
	}

	requestBodyBytes, err := json.Marshal(chatRequest)

	if err != nil {
		log.Errorf(c, "[ollama_large_language_model_adapter.buildJsonRequestBody] failed to marshal request body for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrOperationFailed
	}

	log.Debugf(c, "[ollama_large_language_model_adapter.buildJsonRequestBody] request body is %s", requestBodyBytes)
	return requestBodyBytes, nil
}

func (p *OllamaLargeLanguageModelAdapter) getOllamaRequestUrl() string {
	url := p.OllamaServerURL

	if url[len(url)-1] != '/' {
		url += "/"
	}

	url += ollamaChatCompletionsPath
	return url
}

// NewOllamaLargeLanguageModelProvider creates a new Ollama large language model provider instance
func NewOllamaLargeLanguageModelProvider(llmConfig *settings.LLMConfig, enableResponseLog bool) provider.LargeLanguageModelProvider {
	return common.NewCommonHttpLargeLanguageModelProvider(llmConfig, enableResponseLog, &OllamaLargeLanguageModelAdapter{
		OllamaServerURL:     llmConfig.OllamaServerURL,
		OllamaModelID:       llmConfig.OllamaModelID,
		OllamaThinkingLevel: llmConfig.EnableThinking,
		OllamaNumCtx:        llmConfig.OllamaNumCtx,
		OllamaNumPredict:    llmConfig.OllamaNumPredict,
	})
}
