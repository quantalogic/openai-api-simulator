package streaming

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/quantalogic/openai-api-simulator/pkg/models"
	"github.com/stretchr/testify/require"
)

// fakeFlusher implements http.ResponseWriter + http.Flusher
type fakeFlusher struct {
	builder strings.Builder
}

func (f *fakeFlusher) Header() http.Header         { return http.Header{} }
func (f *fakeFlusher) Write(b []byte) (int, error) { return f.builder.Write(b) }
func (f *fakeFlusher) WriteHeader(statusCode int)  {}
func (f *fakeFlusher) Flush()                      {}

func (f *fakeFlusher) String() string { return f.builder.String() }

func TestStreamCompletion(t *testing.T) {
	handler := NewSSEStreamHandler()

	req := &ChatCompletionRequest{
		Model:    "gpt-sim-1",
		Messages: []models.ChatCompletionMessage{{Role: "user", Content: "Hello"}},
		Stream:   false,
	}

	fw := &fakeFlusher{}
	err := handler.StreamCompletion(context.Background(), fw, req, StreamOptions{IncludeUsage: true, ChunkSize: 3})
	require.NoError(t, err)

	out := fw.String()
	// Should contain chunk and [DONE]
	require.Contains(t, out, "chat.completion.chunk")
	require.Contains(t, out, "[DONE]")
}

func TestStreamCompletion_WithLatencyAndThrottle(t *testing.T) {
	handler := NewSSEStreamHandler()

	req := &ChatCompletionRequest{
		Model:    "gpt-sim-1",
		Messages: []models.ChatCompletionMessage{{Role: "user", Content: "Hello"}},
		Stream:   false,
	}

	fw := &fakeFlusher{}
	// set tiny delays and a high token rate so the test runs fast but still
	// ensures the latency/throttle code path is exercised.
	opts := StreamOptions{IncludeUsage: true, ChunkSize: 3, DelayMin: time.Millisecond, DelayMax: 2 * time.Millisecond, TokensPerSecond: 1000}

	err := handler.StreamCompletion(context.Background(), fw, req, opts)
	require.NoError(t, err)

	out := fw.String()
	require.Contains(t, out, "chat.completion.chunk")
	require.Contains(t, out, "[DONE]")
}
