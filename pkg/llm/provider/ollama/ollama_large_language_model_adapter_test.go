package ollama

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/llm/data"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func TestOllamaLargeLanguageModelAdapter_buildJsonRequestBody_TextualUserPrompt(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{
		OllamaModelID: "test",
	}

	request := &data.LargeLanguageModelRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   []byte("Hello, how are you?"),
	}

	bodyBytes, err := adapter.buildJsonRequestBody(core.NewNullContext(), 0, request, data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)

	var body map[string]interface{}
	err = json.Unmarshal(bodyBytes, &body)
	assert.Nil(t, err)

	assert.Equal(t, "{\"model\":\"test\",\"stream\":false,\"messages\":[{\"role\":\"system\",\"content\":\"You are a helpful assistant.\"},{\"role\":\"user\",\"content\":\"Hello, how are you?\"}],\"format\":\"json\",\"options\":{\"temperature\":0,\"top_p\":1,\"top_k\":1,\"repeat_penalty\":1}}", string(bodyBytes))
}

func TestOllamaLargeLanguageModelAdapter_buildJsonRequestBody_ImageUserPrompt(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{
		OllamaModelID: "test",
	}

	request := &data.LargeLanguageModelRequest{
		SystemPrompt:   "What's in this image?",
		UserPrompt:     []byte("fakedata"),
		UserPromptType: data.LARGE_LANGUAGE_MODEL_REQUEST_PROMPT_TYPE_IMAGE_URL,
	}

	bodyBytes, err := adapter.buildJsonRequestBody(core.NewNullContext(), 0, request, data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)

	var body map[string]interface{}
	err = json.Unmarshal(bodyBytes, &body)
	assert.Nil(t, err)

	assert.Equal(t, "{\"model\":\"test\",\"stream\":false,\"messages\":[{\"role\":\"system\",\"content\":\"What's in this image?\"},{\"role\":\"user\",\"content\":\"\",\"images\":[\"ZmFrZWRhdGE=\"]}],\"format\":\"json\",\"options\":{\"temperature\":0,\"top_p\":1,\"top_k\":1,\"repeat_penalty\":1}}", string(bodyBytes))
}

func TestOllamaLargeLanguageModelAdapter_buildJsonRequestBody_ThinkingHigh(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{
		OllamaModelID:       "test",
		OllamaThinkingLevel: settings.LLMThinkingHigh,
	}

	request := &data.LargeLanguageModelRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   []byte("Hello, how are you?"),
	}

	bodyBytes, err := adapter.buildJsonRequestBody(core.NewNullContext(), 0, request, data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)

	var body map[string]interface{}
	err = json.Unmarshal(bodyBytes, &body)
	assert.Nil(t, err)

	assert.Equal(t, "{\"model\":\"test\",\"stream\":false,\"messages\":[{\"role\":\"system\",\"content\":\"You are a helpful assistant.\"},{\"role\":\"user\",\"content\":\"Hello, how are you?\"}],\"think\":\"high\",\"format\":\"json\",\"options\":{\"temperature\":0,\"top_p\":1,\"top_k\":1,\"repeat_penalty\":1}}", string(bodyBytes))
}

// the sampler has to be pinned on every request: leaving it to the Modelfile is what let a receipt
// import silently lose printed lines and read a price off the neighbouring row
func TestOllamaLargeLanguageModelAdapter_buildJsonRequestBody_AlwaysUsesGreedyDecoding(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{
		OllamaModelID: "test",
	}

	request := &data.LargeLanguageModelRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   []byte("Hello, how are you?"),
	}

	bodyBytes, err := adapter.buildJsonRequestBody(core.NewNullContext(), 0, request, data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)

	var body struct {
		Options *OllamaChatRequestOptions `json:"options"`
	}
	err = json.Unmarshal(bodyBytes, &body)
	assert.Nil(t, err)

	assert.NotNil(t, body.Options)
	assert.Equal(t, float32(0), body.Options.Temperature)
	assert.Equal(t, float32(1), body.Options.TopP)
	assert.Equal(t, int32(1), body.Options.TopK)
	assert.Equal(t, float32(1), body.Options.RepeatPenalty)

	// left to the model unless the deployment configured them
	assert.Equal(t, uint32(0), body.Options.NumCtx)
	assert.Equal(t, uint32(0), body.Options.NumPredict)
}

func TestOllamaLargeLanguageModelAdapter_buildJsonRequestBody_ConfiguredContextAndOutputBudget(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{
		OllamaModelID:    "test",
		OllamaNumCtx:     32768,
		OllamaNumPredict: 4096,
	}

	request := &data.LargeLanguageModelRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   []byte("Hello, how are you?"),
	}

	bodyBytes, err := adapter.buildJsonRequestBody(core.NewNullContext(), 0, request, data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)

	assert.Equal(t, "{\"model\":\"test\",\"stream\":false,\"messages\":[{\"role\":\"system\",\"content\":\"You are a helpful assistant.\"},{\"role\":\"user\",\"content\":\"Hello, how are you?\"}],\"format\":\"json\",\"options\":{\"temperature\":0,\"top_p\":1,\"top_k\":1,\"repeat_penalty\":1,\"num_ctx\":32768,\"num_predict\":4096}}", string(bodyBytes))
}

func TestOllamaLargeLanguageModelAdapter_ParseTextualResponse_ValidJsonResponse(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{}

	response := `{
		"model": "test",
		"created_at": "2025-09-01T01:02:03.456789Z",
		"message": {
			"role": "assistant",
			"content": "This is a test response"
		}
	}`

	result, err := adapter.ParseTextualResponse(core.NewNullContext(), 0, []byte(response), data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)
	assert.Equal(t, "This is a test response", result.Content)
}

func TestOllamaLargeLanguageModelAdapter_ParseTextualResponse_EmptyResponse(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{}

	response := `{
		"model": "test",
		"created_at": "2025-09-01T01:02:03.456789Z",
		"message": {
			"role": "assistant",
			"content": ""
		}
	}`

	result, err := adapter.ParseTextualResponse(core.NewNullContext(), 0, []byte(response), data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.Nil(t, err)
	assert.Equal(t, "", result.Content)
}

func TestOllamaLargeLanguageModelAdapter_ParseTextualResponse_EmptyMessage(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{}

	response := `{
		"model": "test",
		"created_at": "2025-09-01T01:02:03.456789Z",
		"message": {}
	}`

	_, err := adapter.ParseTextualResponse(core.NewNullContext(), 0, []byte(response), data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.EqualError(t, err, "failed to request third party api")
}

func TestOllamaLargeLanguageModelAdapter_ParseTextualResponse_NoContentFieldInMessage(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{}

	response := `{
		"model": "test",
		"created_at": "2025-09-01T01:02:03.456789Z",
		"message": {
			"role": "assistant"
		}
	}`

	_, err := adapter.ParseTextualResponse(core.NewNullContext(), 0, []byte(response), data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.EqualError(t, err, "failed to request third party api")
}

func TestOllamaLargeLanguageModelAdapter_ParseTextualResponse_InvalidJson(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{}

	response := "error"

	_, err := adapter.ParseTextualResponse(core.NewNullContext(), 0, []byte(response), data.LARGE_LANGUAGE_MODEL_RESPONSE_FORMAT_JSON)
	assert.EqualError(t, err, "failed to request third party api")
}

func TestOllamaLargeLanguageModelAdapter_GetOllamaRequestUrl(t *testing.T) {
	adapter := &OllamaLargeLanguageModelAdapter{
		OllamaServerURL: "http://localhost:11434/",
	}
	url := adapter.getOllamaRequestUrl()
	assert.Equal(t, "http://localhost:11434/api/chat", url)

	adapter = &OllamaLargeLanguageModelAdapter{
		OllamaServerURL: "http://localhost:11434",
	}
	url = adapter.getOllamaRequestUrl()
	assert.Equal(t, "http://localhost:11434/api/chat", url)

	adapter = &OllamaLargeLanguageModelAdapter{
		OllamaServerURL: "http://example.com/ollama/",
	}
	url = adapter.getOllamaRequestUrl()
	assert.Equal(t, "http://example.com/ollama/api/chat", url)
}
