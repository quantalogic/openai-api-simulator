package test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-api-simulator/pkg/models"
	"github.com/openai/openai-api-simulator/pkg/server"
	"github.com/stretchr/testify/require"
)

func setupServer() *httptest.Server {
	return httptest.NewServer(server.NewRouter())
}

func TestCompletionEndpoint_NonStreaming(t *testing.T) {
	s := setupServer()
	defer s.Close()

	payload := models.ChatCompletionRequest{
		Model:    "gpt-sim-1",
		Messages: []models.ChatCompletionMessageParam{{Role: "user", Content: "Hello"}},
		Stream:   false,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out models.ChatCompletion
	err = json.NewDecoder(resp.Body).Decode(&out)
	require.NoError(t, err)
	require.Equal(t, "chat.completion", out.Object)
	require.NotEmpty(t, out.Choices)
}

func TestResponseLengthShort(t *testing.T) {
	s := setupServer()
	defer s.Close()

	payload := models.ChatCompletionRequest{
		Model:          "gpt-sim-1",
		Messages:       []models.ChatCompletionMessageParam{{Role: "user", Content: "Hello"}},
		Stream:         false,
		ResponseLength: "short",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var out models.ChatCompletion
	err = json.NewDecoder(resp.Body).Decode(&out)
	require.NoError(t, err)
	require.LessOrEqual(t, len(out.Choices[0].Message.Content), 140)
}

func TestResponseLengthLong(t *testing.T) {
	s := setupServer()
	defer s.Close()

	payload := models.ChatCompletionRequest{
		Model:          "gpt-sim-1",
		Messages:       []models.ChatCompletionMessageParam{{Role: "user", Content: "Please write a long text"}},
		Stream:         false,
		ResponseLength: "long",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var out models.ChatCompletion
	err = json.NewDecoder(resp.Body).Decode(&out)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(out.Choices[0].Message.Content), 360)
}

func TestModelsEndpoint(t *testing.T) {
	s := setupServer()
	defer s.Close()

	resp, err := http.Get(s.URL + "/v1/models")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	_, ok := body["data"]
	require.True(t, ok)
}

func TestLegacyModelsEndpoint(t *testing.T) {
	s := setupServer()
	defer s.Close()

	resp, err := http.Get(s.URL + "/models")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	_, ok := body["data"]
	require.True(t, ok)
}

func TestCompletionEndpoint_StructuredOutput(t *testing.T) {
	s := setupServer()
	defer s.Close()

	schema := map[string]interface{}{
		"type": "json_schema",
		"json_schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
				"age":  map[string]interface{}{"type": "integer"},
			},
			"required": []string{"name"},
		},
	}

	payload := map[string]interface{}{
		"model":           "gpt-sim-1",
		"messages":        []map[string]string{{"role": "user", "content": "Give me a simple JSON with name"}},
		"response_format": schema,
	}

	b, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()

	var out models.ChatCompletion
	err = json.NewDecoder(resp.Body).Decode(&out)
	require.NoError(t, err)
	require.NotEmpty(t, out.Choices)
	require.Contains(t, out.Choices[0].Message.Content, "name")
}

func TestCompletionEndpoint_Streaming(t *testing.T) {
	s := setupServer()
	defer s.Close()

	payload := models.ChatCompletionRequest{
		Model:    "gpt-sim-1",
		Messages: []models.ChatCompletionMessageParam{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	scanner := bufio.NewScanner(resp.Body)
	chunkCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: {") {
			chunkCount++
		}
		if strings.Contains(line, "[DONE]") {
			break
		}
	}
	require.Greater(t, chunkCount, 0)
}

func TestLegacyCompletionEndpoint_NonStreaming(t *testing.T) {
	s := setupServer()
	defer s.Close()

	payload := models.ChatCompletionRequest{
		Model:    "gpt-sim-1",
		Messages: []models.ChatCompletionMessageParam{{Role: "user", Content: "Hello"}},
		Stream:   false,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/chat/completions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out models.ChatCompletion
	err = json.NewDecoder(resp.Body).Decode(&out)
	require.NoError(t, err)
	require.Equal(t, "chat.completion", out.Object)
	require.NotEmpty(t, out.Choices)
}

func TestParallelToolCallsStreaming(t *testing.T) {
	s := setupServer()
	defer s.Close()

	tools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name": "get_weather",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
					"required": []string{"location"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name": "search_web",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
		},
	}

	payload := map[string]interface{}{
		"model":               "gpt-sim-1",
		"messages":            []map[string]string{{"role": "user", "content": "Check weather and search web"}},
		"tools":               tools,
		"stream":              true,
		"parallel_tool_calls": true,
	}

	b, _ := json.Marshal(payload)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	scanner := bufio.NewScanner(resp.Body)
	var toolHeaders int
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "\"tool_calls\"") || strings.Contains(line, "get_weather") || strings.Contains(line, "search_web") {
			toolHeaders++
		}
		if strings.Contains(line, "[DONE]") {
			break
		}
	}
	require.Greater(t, toolHeaders, 0)
}
