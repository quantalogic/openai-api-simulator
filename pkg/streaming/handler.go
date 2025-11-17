package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-api-simulator/pkg/generator"
	"github.com/openai/openai-api-simulator/pkg/models"
	"github.com/openai/openai-api-simulator/pkg/utils"
)

// StreamOptions configures the streaming session.
type StreamOptions struct {
	IncludeUsage      bool
	ChunkSize         int
	// Delay represents an explicit fixed delay after each chunk. Prefer
	// using DelayMin/Max for variance; Delay is kept for backwards
	// compatibility with internal calls.
	Delay             time.Duration

	// DelayMin/DelayMax represent a randomized jitter range applied
	// per-chunk. When set, each chunk will sleep for a uniform random
	// time between DelayMin and DelayMax.
	DelayMin          time.Duration
	DelayMax          time.Duration

	// TokensPerSecond, when >0, throttles output to roughly this
	// token emission rate. It ensures that larger chunks take longer
	// to send and simulates compute throughput.
	TokensPerSecond   float64
	ParallelToolCalls bool
}

// ChatCompletionRequest models the subset of fields we support.
type ChatCompletionRequest struct {
	Model          string                         `json:"model"`
	Messages       []models.ChatCompletionMessage `json:"messages"`
	Tools          []generator.ToolDefinition     `json:"tools"`
	Stream         bool                           `json:"stream"`
	MaxTokens      int64                          `json:"max_tokens"`
	Temperature    float64                        `json:"temperature"`
	ResponseLength string                         `json:"response_length,omitempty"`
}

// SSEStreamHandler emits Server-Sent Events for chat completions.
type SSEStreamHandler struct {
	textGen generator.TextGenerator
	toolGen *generator.ToolCallGenerator
	idGen   *utils.IDGenerator
	// defaults applied when a client does not set values for options.
	defaults *StreamOptions
}

// NewSSEStreamHandler builds a handler backed by default generators.
func NewSSEStreamHandler() *SSEStreamHandler {
	return &SSEStreamHandler{
		textGen: generator.NewCoherentTextGenerator(),
		toolGen: generator.NewToolCallGenerator(),
		idGen:   utils.NewIDGenerator(),
	}
}

// NewSSEStreamHandlerWithDefaults creates a handler with the provided
// default streaming options. If a request does not include `stream_options`
// the handler will apply these defaults.
func NewSSEStreamHandlerWithDefaults(defaults StreamOptions) *SSEStreamHandler {
	return &SSEStreamHandler{
		textGen:  generator.NewCoherentTextGenerator(),
		toolGen:  generator.NewToolCallGenerator(),
		idGen:    utils.NewIDGenerator(),
		defaults: &defaults,
	}
}

// StreamCompletion streams SSE chunks for a completion.
func (h *SSEStreamHandler) StreamCompletion(
	ctx context.Context,
	w http.ResponseWriter,
	req *ChatCompletionRequest,
	opts StreamOptions,
) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	completionID := h.idGen.GenerateID()
	created := time.Now().Unix()

	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 3
	}

	// Merge defaults: for fields not set by the client prefer the server
	// defaults. This only applies to jitter/delay and tokens per second.
	if h.defaults != nil {
		if opts.Delay == 0 {
			opts.Delay = h.defaults.Delay
		}
		if opts.DelayMin == 0 {
			opts.DelayMin = h.defaults.DelayMin
		}
		if opts.DelayMax == 0 {
			opts.DelayMax = h.defaults.DelayMax
		}
		if opts.TokensPerSecond == 0 {
			opts.TokensPerSecond = h.defaults.TokensPerSecond
		}
	}

	var fullText string
	var toolCalls []models.ChatCompletionMessageToolCall
	finishReason := "stop"

	if len(req.Tools) > 0 {
		calls, err := h.toolGen.GenerateToolCalls(ctx, req.Tools, generator.StrategyRandom)
		if err == nil && len(calls) > 0 {
			toolCalls = calls
			finishReason = "tool_calls"
		}
	}

	if len(toolCalls) == 0 {
		// Map friendly response_length to explicit min/max lengths; prefer
		// response length inferred from the message contents when not set.
		minLen, maxLen := MapResponseLengthToRangeForMessages(req.ResponseLength, req.Messages)
		fullText = h.textGen.GenerateText(ctx, minLen, maxLen)
	}

	if len(fullText) > 0 {
		h.streamTextChunks(w, flusher, completionID, created, req.Model, fullText, opts)
	}

	if len(toolCalls) > 0 {
		h.streamToolCallChunks(w, flusher, completionID, created, req.Model, toolCalls, opts)
	}

	h.sendChunk(w, flusher, completionID, created, req.Model, models.ChatCompletionChunkChoice{
		Index:        0,
		Delta:        models.ChatCompletionChunkChoiceDelta{},
		FinishReason: &finishReason,
	}, nil)

	if opts.IncludeUsage {
		promptTokens := utils.EstimateTokens(strings.Join(messageToStrings(req.Messages), " "))
		completionTokens := utils.EstimateTokens(fullText)
		usage := &models.CompletionUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
		chunk := models.ChatCompletionChunk{
			ID:      completionID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []models.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: models.ChatCompletionChunkChoiceDelta{},
			}},
			Usage: usage,
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()

	return nil
}

// mapResponseLengthToRange maps a friendly profile name to a min/max length
// used by the generator. Defaults to medium (120-360) when empty or unknown.
func MapResponseLengthToRange(profile string) (int, int) {
	switch profile {
	case "short":
		return 30, 140
	case "long":
		return 360, 1200
	default:
		return 120, 360
	}
}

// MapResponseLengthToRangeForMessages chooses a min/max length based on the
// content length of the incoming messages. If `profile` is specified it has
// precedence. Otherwise this function uses simple heuristics and a random
// draw to determine a short/medium/long mapping so the simulator output is
// varied and depends on the user's query.
func MapResponseLengthToRangeForMessages(profile string, messages []models.ChatCompletionMessage) (int, int) {
	if profile != "" {
		return MapResponseLengthToRange(profile)
	}

	// Compute average message length
	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	avg := 0
	if len(messages) > 0 {
		avg = total / len(messages)
	}

	// Use randomness to avoid deterministic output; seed with time.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	p := r.Float32()

	// Heuristic thresholds:
	// - very short queries (avg < 50) -> favor short
	// - long queries (avg > 250) -> favor long
	// - otherwise -> mostly medium
	if avg < 50 {
		if p < 0.8 { // 80% short
			return MapResponseLengthToRange("short")
		}
		return MapResponseLengthToRange("medium")
	}

	if avg > 250 {
		if p < 0.8 { // 80% long
			return MapResponseLengthToRange("long")
		}
		return MapResponseLengthToRange("medium")
	}

	// For queries of medium size pick a mixture
	if p < 0.2 {
		return MapResponseLengthToRange("short")
	} else if p < 0.85 {
		return MapResponseLengthToRange("medium")
	}
	return MapResponseLengthToRange("long")
}

func (h *SSEStreamHandler) streamTextChunks(
	w http.ResponseWriter,
	flusher http.Flusher,
	completionID string,
	created int64,
	model string,
	text string,
	opts StreamOptions,
) {
	words := strings.Fields(text)
	for i := 0; i < len(words); i += opts.ChunkSize {
		end := i + opts.ChunkSize
		if end > len(words) {
			end = len(words)
		}
		chunkText := strings.Join(words[i:end], " ")
		if end < len(words) {
			chunkText += " "
		}
		h.sendChunk(w, flusher, completionID, created, model, models.ChatCompletionChunkChoice{
			Index: 0,
			Delta: models.ChatCompletionChunkChoiceDelta{
				Role:    "assistant",
				Content: chunkText,
			},
		}, nil)
		// Compute delay: prefer DelayMin/DelayMax randomness; if not
		// set, fall back to Delay fixed value. In addition, honor
		// TokensPerSecond throttling which may extend the sleep to
		// respect emission rate.
		if opts.DelayMin > 0 || opts.DelayMax > 0 {
			// ensure min <= max
			min := opts.DelayMin
			max := opts.DelayMax
			if max < min {
				max = min
			}
			// random in [min, max]
			d := time.Duration(rand.Int63n(int64(max-min)+1)) + min
			time.Sleep(d)
		} else if opts.Delay > 0 {
			time.Sleep(opts.Delay)
		}

		// Throttle by approximate token rate if requested. This sleep
		// enforces a minimum duration; it does not reduce random jitter
		// above.
		if opts.TokensPerSecond > 0 {
			tokens := utils.EstimateTokens(chunkText)
			// tokens/sec -> seconds
			dur := time.Duration(float64(tokens)/opts.TokensPerSecond*float64(time.Second))
			// if the tokens-based sleep is larger than the previous one
			// we need to wait the extra time.
			if dur > 0 {
				time.Sleep(dur)
			}
		}
	}
}

func (h *SSEStreamHandler) streamToolCallChunks(
	w http.ResponseWriter,
	flusher http.Flusher,
	completionID string,
	created int64,
	model string,
	toolCalls []models.ChatCompletionMessageToolCall,
	opts StreamOptions,
) {
	if opts.ParallelToolCalls {
		// Use a writer goroutine to safely serialize writes to the ResponseWriter
		type item struct {
			chunk models.ChatCompletionChunk
		}

		ch := make(chan item, len(toolCalls)*4)
		var wg sync.WaitGroup

		// writer goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				data, _ := json.Marshal(it.chunk)
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
			}
		}()

		// launch per-tool goroutines that push chunks to channel
		var workerWG sync.WaitGroup
		for idx, call := range toolCalls {
			workerWG.Add(1)
			go func(idx int, call models.ChatCompletionMessageToolCall) {
				defer workerWG.Done()

				chunk := models.ChatCompletionChunk{
					ID:      completionID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []models.ChatCompletionChunkChoice{{
						Index: int64(idx),
						Delta: models.ChatCompletionChunkChoiceDelta{
							ToolCalls: []models.ChatCompletionChunkToolCall{{
								Index: int64(idx),
								ID:    call.ID,
								Type:  call.Type,
								Function: models.ChatCompletionChunkToolCallFunction{
									Name: call.Function.Name,
								},
							}},
						},
					}},
				}
				ch <- item{chunk: chunk}

				args := call.Function.Arguments
				if args != "" {
					for j := 0; j < len(args); j += 20 {
						end := j + 20
						if end > len(args) {
							end = len(args)
						}
						chunk2 := models.ChatCompletionChunk{
							ID:      completionID,
							Object:  "chat.completion.chunk",
							Created: created,
							Model:   model,
							Choices: []models.ChatCompletionChunkChoice{{
								Index: int64(idx),
								Delta: models.ChatCompletionChunkChoiceDelta{
									ToolCalls: []models.ChatCompletionChunkToolCall{{
										Index: int64(idx),
										Function: models.ChatCompletionChunkToolCallFunction{
											Arguments: args[j:end],
										},
									}},
								},
							}},
						}
						ch <- item{chunk: chunk2}
						// Writer goroutine does not know what the token
						// content will be, so the worker sleeps between
						// chunks to simulate the tool output sustain.
						// Respect random jitter (min/max), fixed Delay, and
						// the token throttle.
						if opts.DelayMin > 0 || opts.DelayMax > 0 {
							min := opts.DelayMin
							max := opts.DelayMax
							if max < min {
								max = min
							}
							d := time.Duration(rand.Int63n(int64(max-min)+1)) + min
							time.Sleep(d)
						} else if opts.Delay > 0 {
							time.Sleep(opts.Delay)
						}
						if opts.TokensPerSecond > 0 {
							tokens := utils.EstimateTokens(args[j:end])
							dur := time.Duration(float64(tokens)/opts.TokensPerSecond*float64(time.Second))
							if dur > 0 {
								time.Sleep(dur)
							}
						}
					}
				}
			}(idx, call)
		}

		workerWG.Wait()
		close(ch)
		wg.Wait()
		return
	}

	for idx, call := range toolCalls {
		h.sendChunk(w, flusher, completionID, created, model, models.ChatCompletionChunkChoice{
			Index: int64(idx),
			Delta: models.ChatCompletionChunkChoiceDelta{
				ToolCalls: []models.ChatCompletionChunkToolCall{
					{
						Index: int64(idx),
						ID:    call.ID,
						Type:  call.Type,
						Function: models.ChatCompletionChunkToolCallFunction{
							Name: call.Function.Name,
						},
					},
				},
			},
		}, nil)
		args := call.Function.Arguments
		if args != "" {
			for j := 0; j < len(args); j += 20 {
				end := j + 20
				if end > len(args) {
					end = len(args)
				}
				h.sendChunk(w, flusher, completionID, created, model, models.ChatCompletionChunkChoice{
					Index: int64(idx),
					Delta: models.ChatCompletionChunkChoiceDelta{
						ToolCalls: []models.ChatCompletionChunkToolCall{{
							Index: int64(idx),
							Function: models.ChatCompletionChunkToolCallFunction{
								Arguments: args[j:end],
							},
						}},
					},
				}, nil)
					// Sleep by jitter / fixed delay then throttle by token rate
					if opts.DelayMin > 0 || opts.DelayMax > 0 {
						min := opts.DelayMin
						max := opts.DelayMax
						if max < min {
							max = min
						}
						d := time.Duration(rand.Int63n(int64(max-min)+1)) + min
						time.Sleep(d)
					} else if opts.Delay > 0 {
						time.Sleep(opts.Delay)
					}
					if opts.TokensPerSecond > 0 {
						tokens := utils.EstimateTokens(args[j:end])
						dur := time.Duration(float64(tokens)/opts.TokensPerSecond*float64(time.Second))
						if dur > 0 {
							time.Sleep(dur)
						}
					}
			}
		}
	}
}

func (h *SSEStreamHandler) sendChunk(
	w http.ResponseWriter,
	flusher http.Flusher,
	completionID string,
	created int64,
	model string,
	choice models.ChatCompletionChunkChoice,
	usage *models.CompletionUsage,
) {
	chunk := models.ChatCompletionChunk{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []models.ChatCompletionChunkChoice{choice},
		Usage:   usage,
	}
	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
}

func messageToStrings(messages []models.ChatCompletionMessage) []string {
	var result []string
	for _, msg := range messages {
		if msg.Content != "" {
			result = append(result, msg.Content)
		}
	}
	return result
}
