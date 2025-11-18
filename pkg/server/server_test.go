package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quantalogic/openai-api-simulator/pkg/models"
	"github.com/quantalogic/openai-api-simulator/pkg/streaming"
	"github.com/stretchr/testify/require"
)

func TestRouter_UsesStreamDefaults(t *testing.T) {
	defaults := streaming.StreamOptions{DelayMin: time.Millisecond, DelayMax: 2 * time.Millisecond, TokensPerSecond: 1000}
	s := httptest.NewServer(NewRouterWithStreamDefaults(defaults))
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
