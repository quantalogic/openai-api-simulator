package models

import (
	"encoding/json"
)

// ChatCompletion represents the response from the API for a chat completion request
type ChatCompletion struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             CompletionUsage        `json:"usage"`
	SystemFingerprint string                 `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChunk represents a chunk in a streaming response
type ChatCompletionChunk struct {
	ID                string                      `json:"id"`
	Object            string                      `json:"object"`
	Created           int64                       `json:"created"`
	Model             string                      `json:"model"`
	Choices           []ChatCompletionChunkChoice `json:"choices"`
	Usage             *CompletionUsage            `json:"usage,omitempty"`
	SystemFingerprint string                      `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChoice represents a choice in a completion response
type ChatCompletionChoice struct {
	Index        int64                         `json:"index"`
	Message      ChatCompletionMessage         `json:"message"`
	FinishReason string                        `json:"finish_reason"`
	Logprobs     *ChatCompletionChoiceLogprobs `json:"logprobs,omitempty"`
}

// ChatCompletionChunkChoice represents a choice in a streaming chunk
type ChatCompletionChunkChoice struct {
	Index        int64                          `json:"index"`
	Delta        ChatCompletionChunkChoiceDelta `json:"delta"`
	FinishReason *string                        `json:"finish_reason"`
	Logprobs     *ChatCompletionChoiceLogprobs  `json:"logprobs,omitempty"`
}

// ChatCompletionChunkChoiceDelta represents the delta in a streaming chunk
type ChatCompletionChunkChoiceDelta struct {
	Role      string                        `json:"role,omitempty"`
	Content   string                        `json:"content,omitempty"`
	ToolCalls []ChatCompletionChunkToolCall `json:"tool_calls,omitempty"`
}

// ChatCompletionChunkToolCall represents a tool call in a streaming chunk
type ChatCompletionChunkToolCall struct {
	Index    int64                               `json:"index,omitempty"`
	ID       string                              `json:"id,omitempty"`
	Type     string                              `json:"type,omitempty"`
	Function ChatCompletionChunkToolCallFunction `json:"function,omitempty"`
}

// ChatCompletionChunkToolCallFunction represents the function in a streaming tool call
type ChatCompletionChunkToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ChatCompletionMessage represents a message in a completion response
type ChatCompletionMessage struct {
	Role      string                          `json:"role"`
	Content   string                          `json:"content"`
	ToolCalls []ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
	Refusal   string                          `json:"refusal,omitempty"`
}

// ChatCompletionMessageToolCall represents a tool call made by the model
type ChatCompletionMessageToolCall struct {
	ID       string                                `json:"id"`
	Type     string                                `json:"type"`
	Function ChatCompletionMessageToolCallFunction `json:"function"`
}

// ChatCompletionMessageToolCallFunction represents the function of a tool call
type ChatCompletionMessageToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionChoiceLogprobs represents log probability information
type ChatCompletionChoiceLogprobs struct {
	Content []ChatCompletionTokenLogprob `json:"content"`
	Refusal []ChatCompletionTokenLogprob `json:"refusal"`
}

// ChatCompletionTokenLogprob represents token log probability information
type ChatCompletionTokenLogprob struct {
	Token       string                          `json:"token"`
	Logprob     float64                         `json:"logprob"`
	Bytes       []int64                         `json:"bytes"`
	TopLogprobs []ChatCompletionTokenLogprobTop `json:"top_logprobs"`
}

// ChatCompletionTokenLogprobTop represents top log probability for a token
type ChatCompletionTokenLogprobTop struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int64 `json:"bytes"`
}

// CompletionUsage represents token usage information
type CompletionUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// ChatCompletionMessageParam represents a message parameter in a request
type ChatCompletionMessageParam struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	Name       string            `json:"name,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []json.RawMessage `json:"tool_calls,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Type     string              `json:"type"`
	Function *FunctionDefinition `json:"function,omitempty"`
}

// FunctionDefinition represents a function tool definition
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// JSONSchema represents a JSON schema
type JSONSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]PropertyDef `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Items      *PropertyDef           `json:"items,omitempty"`
	Enum       []interface{}          `json:"enum,omitempty"`
}

// PropertyDef represents a property definition in a schema
type PropertyDef struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]PropertyDef `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Items       *PropertyDef           `json:"items,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
}

// ChatCompletionRequest represents the full request body
type ChatCompletionRequest struct {
	Model               string                       `json:"model"`
	Messages            []ChatCompletionMessageParam `json:"messages"`
	Tools               []Tool                       `json:"tools,omitempty"`
	ToolChoice          interface{}                  `json:"tool_choice,omitempty"`
	Stream              bool                         `json:"stream,omitempty"`
	StreamOptions       *StreamOptions               `json:"stream_options,omitempty"`
	Temperature         *float64                     `json:"temperature,omitempty"`
	TopP                *float64                     `json:"top_p,omitempty"`
	MaxTokens           *int64                       `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int64                       `json:"max_completion_tokens,omitempty"`
	FrequencyPenalty    *float64                     `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64                     `json:"presence_penalty,omitempty"`
	N                   *int64                       `json:"n,omitempty"`
	Seed                *int64                       `json:"seed,omitempty"`
	Logprobs            *bool                        `json:"logprobs,omitempty"`
	TopLogprobs         *int64                       `json:"top_logprobs,omitempty"`
	ResponseFormat      interface{}                  `json:"response_format,omitempty"`
	Modalities          []string                     `json:"modalities,omitempty"`
	ParallelToolCalls   *bool                        `json:"parallel_tool_calls,omitempty"`
	ResponseLength      string                       `json:"response_length,omitempty"`
}

// StreamOptions represents streaming options
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`

	// DelayMinMs/DelayMaxMs allow configuration of a randomized per-chunk
	// latency (milliseconds). If neither is set, the simulator does not
	// inject additional delay. If only one is set, that value is used
	// as a fixed delay. These are useful for simulating network/compute
	// variability in a model.
	DelayMinMs int `json:"delay_min_ms,omitempty"`
	DelayMaxMs int `json:"delay_max_ms,omitempty"`

	// TokensPerSecond controls a token-rate throttle for streaming
	// chunks. When >0, the simulator will sleep between chunks so the
	// effective emission rate approximates tokens per second. This
	// simulates model compute throughput.
	TokensPerSecond float64 `json:"tokens_per_second,omitempty"`
}
