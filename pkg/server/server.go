package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/quantalogic/openai-api-simulator/pkg/generator"
	"github.com/quantalogic/openai-api-simulator/pkg/models"
	"github.com/quantalogic/openai-api-simulator/pkg/streaming"
	"github.com/quantalogic/openai-api-simulator/pkg/utils"
)

// Map incoming request types to streaming request types.
func toStreamingRequest(in models.ChatCompletionRequest) *streaming.ChatCompletionRequest {
	req := &streaming.ChatCompletionRequest{
		Model: in.Model,
	}

	// Convert messages
	for _, m := range in.Messages {
		req.Messages = append(req.Messages, models.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Tools: convert models.Tool -> generator.ToolDefinition
	for _, t := range in.Tools {
		var fd models.FunctionDefinition
		if t.Function != nil {
			fd = *t.Function
		}
		td := generator.ToolDefinition{
			Function: fd,
			Type:     t.Type,
		}
		req.Tools = append(req.Tools, td)
	}

	req.ResponseLength = in.ResponseLength
	return req
}

// NewRouter returns an http.Handler that exposes simulated endpoints.
func NewRouter() http.Handler {
	return NewRouterWithStreamDefaults(streaming.StreamOptions{}, "")
}

// NewRouterWithStreamDefaults returns a router that applies the provided
// defaults when an incoming request does not supply `stream_options`.
func NewRouterWithStreamDefaults(defaults streaming.StreamOptions, defaultResponseLength string) http.Handler {
	mux := http.NewServeMux()

	sseHandler := streaming.NewSSEStreamHandlerWithDefaults(defaults)

	// Chat completion handler: register both the OpenAI v1 path and the older
	// base path that some UIs (like Open Web UI) use. This ensures the
	// simulator is compatible with both styles.
	chatHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var in models.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
			return
		}

		if in.Stream {
			parallel := false
			if in.ParallelToolCalls != nil {
				parallel = *in.ParallelToolCalls
			}
			streamOpts := streaming.StreamOptions{IncludeUsage: false, ChunkSize: 3, ParallelToolCalls: parallel}
			if in.StreamOptions != nil {
				streamOpts.IncludeUsage = in.StreamOptions.IncludeUsage
				// map jitter/delay range
				if in.StreamOptions.DelayMinMs > 0 {
					streamOpts.DelayMin = time.Duration(in.StreamOptions.DelayMinMs) * time.Millisecond
				}
				if in.StreamOptions.DelayMaxMs > 0 {
					streamOpts.DelayMax = time.Duration(in.StreamOptions.DelayMaxMs) * time.Millisecond
				}
				if in.StreamOptions.TokensPerSecond > 0 {
					streamOpts.TokensPerSecond = in.StreamOptions.TokensPerSecond
				}
			}
			_ = sseHandler.StreamCompletion(r.Context(), w, toStreamingRequest(in), streamOpts)
			return
		}

		// Non-streaming: support structured outputs or plain text
		// Parse response_format for JSON schema
		var structured string
		if in.ResponseFormat != nil {
			// try to treat as map[string]interface{}
			if rf, ok := in.ResponseFormat.(map[string]interface{}); ok {
				if rf["type"] == "json_schema" {
					// try to extract 'json_schema' property
					if js, ok := rf["json_schema"]; ok {
						// re-marshal and decode into models.JSONSchema
						b, _ := json.Marshal(js)
						var schema models.JSONSchema
						if err := json.Unmarshal(b, &schema); err == nil {
							gen := generator.NewToolCallGenerator()
							if out, err := gen.GenerateStructuredOutput(schema); err == nil {
								structured = out
							}
						}
					}
				}
			}
		}

		text := structured
		if text == "" {
			// Convert to streaming request so we can use the same message
			// normalization heuristics for response length (chosen using
			// `response_length` or inferred from the input messages).
			sreq := toStreamingRequest(in)
			// If a default response length is configured and the client did
			// not specify one, set it so the streaming generator uses the
			// configured default.
			if sreq.ResponseLength == "" && defaultResponseLength != "" {
				sreq.ResponseLength = defaultResponseLength
			}
			// If a default response length is configured and the client did
			// not specify one, use that default; otherwise fall back to
			// inferred length.
			profile := in.ResponseLength
			if profile == "" && defaultResponseLength != "" {
				profile = defaultResponseLength
			}
			minLen, maxLen := streaming.MapResponseLengthToRangeForMessages(profile, sreq.Messages)
			text = generator.NewCoherentTextGenerator().GenerateText(r.Context(), minLen, maxLen)
		}
		id := utils.NewIDGenerator().GenerateID()
		created := int64(0)

		if in.MaxTokens != nil {
			_ = *in.MaxTokens
		}

		// Build minimal ChatCompletion
		choice := models.ChatCompletionChoice{
			Index: 0,
			Message: models.ChatCompletionMessage{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: "stop",
		}

		resp := models.ChatCompletion{
			ID:      id,
			Object:  "chat.completion",
			Created: created,
			Model:   in.Model,
			Choices: []models.ChatCompletionChoice{choice},
			Usage:   models.CompletionUsage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
	mux.HandleFunc("/v1/chat/completions", chatHandler)
	mux.HandleFunc("/chat/completions", chatHandler)

	// Quick model listing endpoint
	modelsHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		modelsList := []map[string]string{
			{"id": "gpt-sim-1", "object": "model"},
			{"id": "gpt-4o", "object": "model"},
			{"id": "gpt-3.5-turbo", "object": "model"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": modelsList})
	}
	mux.HandleFunc("/v1/models", modelsHandler)
	mux.HandleFunc("/models", modelsHandler)

	// Root/home endpoint and health check. This makes it easier to confirm the
	// simulator is up when browsing directly or when other services probe
	// the host root (Open Web UI may hit root for diagnostics).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only respond at root path; leave other paths to their handlers
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "openai-api-simulator"})
	})

	// Health endpoint for any readiness/liveness checks.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	// Wrap with a simple logger so that requests are visible in container logs,
	// this helps diagnose 500s from clients like Open Web UI when they fail to
	// reach the simulator.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		mux.ServeHTTP(w, r)
	})
}
