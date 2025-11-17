# ADR-0001: Building an OpenAI API Simulator in Go

**Date:** November 17, 2025  
**Status:** PROPOSED  
**Decision Makers:** Engineering Team  
**Impact Level:** HIGH - Reputation-critical project

---

## 1. Executive Summary

This ADR documents the architectural decisions for building a **mock OpenAI API server in Go** that simulates OpenAI's chat completion endpoints without requiring a real LLM. The simulator generates coherent random English sentences of variable lengths while maintaining full compatibility with the latest OpenAI API specifications including streaming, tool calling, structured outputs, and audio support.

### Core Value Proposition
- **Zero LLM dependency**: Faster, cheaper development/testing
- **API-compliant**: Supports latest OpenAI features (GPT-4o, streaming, tools, audio)
- **Production-ready**: Thread-safe, performant, well-tested
- **Integration-proof**: Drop-in replacement for testing OpenAI-dependent applications

---

## 2. Problem Statement

### Challenges Addressed
1. **Cost & Latency**: Real OpenAI API calls are expensive ($0.005-$0.30 per 1K tokens) and slow for testing
2. **Quota Limitations**: Development teams hit rate limits during active development
3. **Test Flakiness**: Non-deterministic API responses complicate testing
4. **Cold Start Issues**: CI/CD pipelines need instant responses for rapid feedback
5. **Feature Coverage**: Need to support cutting-edge OpenAI features (streaming, parallel tool calls, structured outputs, audio)

### Solution Requirements
- Must generate **syntactically correct responses** matching OpenAI's JSON schema
- Must support **streaming with Server-Sent Events (SSE)**
- Must handle **tool calling with parallel execution**
- Must implement **structured outputs validation**
- Must be **API-compatible** with current OpenAI specifications
- Must provide **deterministic responses** for reproducible testing

---

## 3. Solution Design

### 3.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│         OpenAI API Simulator (Go)                       │
├─────────────────────────────────────────────────────────┤
│  HTTP Server (net/http or gin-gonic)                    │
│  ├─ POST /v1/chat/completions                          │
│  ├─ POST /v1/chat/completions/stream                   │
│  └─ POST /v1/chat/completions (with stream=true)       │
├─────────────────────────────────────────────────────────┤
│  Request Parser & Validator                            │
│  ├─ Message parsing                                    │
│  ├─ Tool/Function definition validation                │
│  ├─ Parameter validation                               │
│  └─ Streaming options handling                         │
├─────────────────────────────────────────────────────────┤
│  Response Generator Engine                             │
│  ├─ Sentence generator (random coherent text)          │
│  ├─ Tool call generator (based on provided tools)      │
│  ├─ Structured output formatter                        │
│  └─ Audio response simulator                           │
├─────────────────────────────────────────────────────────┤
│  Streaming Manager                                     │
│  ├─ SSE chunked encoding                              │
│  ├─ Tool call streaming                               │
│  ├─ Token-by-token simulation                         │
│  └─ Usage tracking                                    │
├─────────────────────────────────────────────────────────┤
│  Data Models (OpenAI-compatible)                       │
│  ├─ ChatCompletion                                    │
│  ├─ ChatCompletionChunk (streaming)                  │
│  ├─ ChatCompletionMessage                            │
│  ├─ Tool/ToolCall definitions                        │
│  └─ CompletionUsage                                  │
└─────────────────────────────────────────────────────────┘
```

### 3.2 Implementation Layers

#### A. Data Model Layer
**File:** `pkg/models/chat_completion.go`

Implement all OpenAI response structures from the official SDK:
```go
type ChatCompletion struct {
    ID                string
    Object            string
    Created           int64
    Model             string
    Choices           []ChatCompletionChoice
    Usage             CompletionUsage
    SystemFingerprint string
}

type ChatCompletionChunk struct {
    ID      string
    Object  string
    Created int64
    Model   string
    Choices []ChatCompletionChunkChoice
    Usage   *CompletionUsage // nil except final chunk
}

type ChatCompletionChoice struct {
    Index        int64
    Message      ChatCompletionMessage
    FinishReason string // "stop", "tool_calls", "length", etc.
    Logprobs     *ChatCompletionChoiceLogprobs
}

type ChatCompletionMessage struct {
    Role      string                       // "assistant"
    Content   string                       // Generated text
    ToolCalls []ChatCompletionMessageToolCall
    Refusal   string
}

type ChatCompletionMessageToolCall struct {
    ID       string
    Type     string // "function"
    Function ChatCompletionToolCallFunction
}

type ChatCompletionToolCallFunction struct {
    Name      string
    Arguments string // JSON string
}

type CompletionUsage struct {
    PromptTokens     int64
    CompletionTokens int64
    TotalTokens      int64
}
```

**Reference:** [OpenAI Go SDK - chatcompletion.go](https://github.com/openai/openai-go/blob/main/chatcompletion.go)

#### B. Text Generation Engine
**File:** `pkg/generator/text_generator.go`

Generate coherent, variable-length English sentences:

```go
type TextGenerator interface {
    // GenerateText generates variable-length coherent English text
    GenerateText(ctx context.Context, minLength, maxLength int) string
    
    // GenerateChunk generates a single streaming chunk (variable tokens)
    GenerateChunk(ctx context.Context) string
}

type CoherentTextGenerator struct {
    // Predefined sentence templates for coherence
    templates []string
    // Random word bank for variation
    nouns, verbs, adjectives []string
    // Seed for determinism (optional)
    seed int64
}

func (g *CoherentTextGenerator) GenerateText(ctx context.Context, minLength, maxLength int) string {
    // 1. Randomly select 2-5 coherent sentence templates
    // 2. Inject random words from word bank while maintaining grammar
    // 3. Combine into multi-sentence response of specified length
    // 4. Ensure output is between minLength and maxLength characters
    
    // Example output:
    // "The intelligent software engineer developed a robust solution yesterday. 
    //  This approach demonstrates strong architectural patterns. 
    //  Furthermore, the team successfully implemented comprehensive testing strategies."
}

func (g *CoherentTextGenerator) GenerateChunk(ctx context.Context) string {
    // Generate single word or 2-3 word phrase for streaming simulation
}
```

**Core Features:**
- **Templates for coherence**: Pre-crafted sentence structures
- **Word banks**: Nouns, verbs, adjectives organized by category
- **Grammar awareness**: Proper subject-verb agreement, articles
- **Length control**: Respect min/max character/token constraints
- **Variability**: Never repeat exact same sequence twice

**Example Templates:**
```go
var templates = []string{
    "The {adj} {noun} {verb} {adv}.",
    "Today, the {noun} {verb} with {adj} {noun}.",
    "{Adj} {noun} {verb} because {noun} {verb}.",
    "In conclusion, this {adj} approach {verb} {adv}.",
}
```

#### C. Tool Call Generator
**File:** `pkg/generator/tool_generator.go`

Generate realistic tool calls based on provided tool definitions:

```go
type ToolCallGenerator interface {
    // GenerateToolCalls creates realistic tool call responses
    GenerateToolCalls(
        ctx context.Context,
        tools []ToolDefinition,
        userMessage string,
        strategy ToolCallStrategy,
    ) ([]ChatCompletionMessageToolCall, error)
}

type ToolCallStrategy string
const (
    StrategyRandom     ToolCallStrategy = "random"      // Random tool from list
    StrategyContextual ToolCallStrategy = "contextual"  // Keyword matching
    StrategySequence   ToolCallStrategy = "sequence"    // Multiple tools in order
)

type ToolDefinition struct {
    Name        string
    Description string
    Parameters  JSONSchema
}

type JSONSchema struct {
    Type       string
    Properties map[string]PropertyDef
    Required   []string
}

func (g *ToolCallGenerator) GenerateToolCalls(
    ctx context.Context,
    tools []ToolDefinition,
    userMessage string,
    strategy ToolCallStrategy,
) ([]ChatCompletionMessageToolCall, error) {
    
    var toolCalls []ChatCompletionMessageToolCall
    
    switch strategy {
    case StrategyRandom:
        // Pick random tools, generate plausible JSON arguments
        tool := tools[rand.Intn(len(tools))]
        toolCalls = append(toolCalls, g.generateCall(tool))
        
    case StrategyContextual:
        // Match tool names/descriptions to keywords in userMessage
        for _, tool := range tools {
            if shouldCallTool(userMessage, tool) {
                toolCalls = append(toolCalls, g.generateCall(tool))
            }
        }
        
    case StrategySequence:
        // Generate calls for all provided tools
        for _, tool := range tools {
            toolCalls = append(toolCalls, g.generateCall(tool))
        }
    }
    
    return toolCalls, nil
}

func (g *ToolCallGenerator) generateCall(tool ToolDefinition) ChatCompletionMessageToolCall {
    // Generate valid JSON arguments matching tool's schema
    args := g.generateJSONArguments(tool.Parameters)
    
    return ChatCompletionMessageToolCall{
        ID:   generateUUID(),
        Type: "function",
        Function: ChatCompletionToolCallFunction{
            Name:      tool.Name,
            Arguments: args,
        },
    }
}

func (g *ToolCallGenerator) generateJSONArguments(schema JSONSchema) string {
    // Create valid JSON matching schema requirements:
    // - Include all required properties
    // - Generate appropriate types (string, number, boolean, object)
    // - Respect enum constraints if present
    // - Handle nested objects recursively
}
```

**Implementation Features:**
- **Schema validation**: Ensure generated arguments match tool schema
- **Deterministic mode**: Seed-based generation for reproducible tests
- **Smart strategy selection**: Context-aware tool calling
- **Proper ID generation**: UUID-based tool call IDs
- **Type correctness**: Match JSON schema types precisely

#### D. Streaming Response Handler
**File:** `pkg/streaming/stream_handler.go`

Implement Server-Sent Events (SSE) streaming compatible with OpenAI spec:

```go
type StreamHandler interface {
    // StreamCompletion sends streamed response chunks via SSE
    StreamCompletion(
        ctx context.Context,
        w http.ResponseWriter,
        req ChatCompletionRequest,
        opts StreamOptions,
    ) error
}

type StreamOptions struct {
    IncludeUsage bool // Include token usage in final chunk
    ChunkSize    int  // Number of words per chunk
    Delay        time.Duration // Simulate latency between chunks
}

type SSEStreamHandler struct {
    generator TextGenerator
    toolGen   ToolCallGenerator
}

func (h *SSEStreamHandler) StreamCompletion(
    ctx context.Context,
    w http.ResponseWriter,
    req ChatCompletionRequest,
    opts StreamOptions,
) error {
    // 1. Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    // 2. Generate full response (text or tool calls)
    var fullText string
    var toolCalls []ChatCompletionMessageToolCall
    
    if hasTools(req.Tools) && shouldCallTools(req) {
        toolCalls, _ = h.toolGen.GenerateToolCalls(ctx, req.Tools, lastMessage(req), StrategyRandom)
        fullText = "" // No text when calling tools
    } else {
        fullText = h.generator.GenerateText(ctx, 100, 500)
    }
    
    // 3. Stream chunks with SSE format
    wordCount := 0
    totalTokens := len(tokenize(fullText))
    
    for chunk := range h.generateChunks(fullText, opts.ChunkSize) {
        // Send SSE delta
        delta := ChatCompletionChunkChoice{
            Index: 0,
            Delta: ChatCompletionChunkChoiceDelta{
                Role:    "assistant",
                Content: chunk,
            },
        }
        
        chunkResp := ChatCompletionChunk{
            ID:      generateID(),
            Object:  "chat.completion.chunk",
            Created: time.Now().Unix(),
            Model:   req.Model,
            Choices: []ChatCompletionChunkChoice{delta},
        }
        
        // Serialize and send
        data, _ := json.Marshal(chunkResp)
        fmt.Fprintf(w, "data: %s\n\n", string(data))
        w.Flush()
        
        if opts.Delay > 0 {
            time.Sleep(opts.Delay)
        }
    }
    
    // 4. Send final chunk with usage info
    if opts.IncludeUsage {
        finalChunk := ChatCompletionChunk{
            ID:      generateID(),
            Object:  "chat.completion.chunk",
            Created: time.Now().Unix(),
            Model:   req.Model,
            Choices: []ChatCompletionChunkChoice{{
                Index:        0,
                FinishReason: "stop",
            }},
            Usage: &CompletionUsage{
                PromptTokens:     estimateTokens(req.Messages),
                CompletionTokens: totalTokens,
                TotalTokens:      estimateTokens(req.Messages) + totalTokens,
            },
        }
        data, _ := json.Marshal(finalChunk)
        fmt.Fprintf(w, "data: %s\n\n", string(data))
    }
    
    // 5. Send done signal
    fmt.Fprint(w, "data: [DONE]\n\n")
    w.Flush()
    
    return nil
}

func (h *SSEStreamHandler) generateChunks(text string, chunkSize int) <-chan string {
    ch := make(chan string)
    go func() {
        defer close(ch)
        words := strings.Fields(text)
        for i := 0; i < len(words); i += chunkSize {
            end := i + chunkSize
            if end > len(words) {
                end = len(words)
            }
            ch <- strings.Join(words[i:end], " ") + " "
        }
    }()
    return ch
}
```

**OpenAI Specification References:**
- [Streaming Responses Documentation](https://github.com/openai/openai-go#streaming-responses)
- SSE Format: `data: {json}\n\n` followed by `data: [DONE]\n\n`
- Each chunk must have structure: `ChatCompletionChunk` with delta
- Final chunk includes `usage` statistics (if `stream_options.include_usage: true`)

#### E. Request Handler & HTTP Server
**File:** `pkg/api/handler.go`

```go
type ChatCompletionHandler struct {
    textGen  TextGenerator
    toolGen  ToolCallGenerator
    streamer StreamHandler
}

func (h *ChatCompletionHandler) HandleCreateCompletion(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var req ChatCompletionNewParams
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // 2. Validate required fields
    if err := validateRequest(req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 3. Handle streaming vs non-streaming
    if req.Stream {
        h.streamer.StreamCompletion(r.Context(), w, req, StreamOptions{
            IncludeUsage: req.StreamOptions.IncludeUsage,
        })
        return
    }
    
    // 4. Generate complete response
    var content string
    var toolCalls []ChatCompletionMessageToolCall
    var finishReason string
    
    if len(req.Tools) > 0 && shouldCallTool(req) {
        toolCalls, _ = h.toolGen.GenerateToolCalls(
            r.Context(),
            convertToToolDefs(req.Tools),
            lastMessage(req.Messages),
            StrategyRandom,
        )
        finishReason = "tool_calls"
    } else {
        content = h.textGen.GenerateText(r.Context(), 100, 500)
        finishReason = "stop"
    }
    
    // 5. Build response
    response := ChatCompletion{
        ID:      "chatcmpl-" + generateID(),
        Object:  "chat.completion",
        Created: time.Now().Unix(),
        Model:   req.Model,
        Choices: []ChatCompletionChoice{{
            Index:        0,
            Message: ChatCompletionMessage{
                Role:      "assistant",
                Content:   content,
                ToolCalls: toolCalls,
            },
            FinishReason: finishReason,
        }},
        Usage: CompletionUsage{
            PromptTokens:     estimateTokens(req.Messages),
            CompletionTokens: estimateTokens(content),
            TotalTokens:      estimateTokens(req.Messages) + estimateTokens(content),
        },
    }
    
    // 6. Send response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func validateRequest(req ChatCompletionNewParams) error {
    if len(req.Messages) == 0 {
        return fmt.Errorf("messages field is required")
    }
    if req.Model == "" {
        return fmt.Errorf("model field is required")
    }
    if req.MaxTokens > 0 && req.MaxTokens > 128000 {
        return fmt.Errorf("max_tokens exceeds model maximum")
    }
    return nil
}
```

**File:** `pkg/api/server.go`

```go
type Server struct {
    router  *http.ServeMux
    handler *ChatCompletionHandler
    addr    string
}

func NewServer(addr string) *Server {
    handler := &ChatCompletionHandler{
        textGen:  NewCoherentTextGenerator(),
        toolGen:  NewToolCallGenerator(),
        streamer: NewSSEStreamHandler(),
    }
    
    server := &Server{
        router:  http.NewServeMux(),
        handler: handler,
        addr:    addr,
    }
    
    // Register routes
    server.router.HandleFunc("POST /v1/chat/completions", handler.HandleCreateCompletion)
    server.router.HandleFunc("POST /v1/models", handleListModels)
    
    return server
}

func (s *Server) Start() error {
    return http.ListenAndServe(s.addr, s.router)
}
```

---

## 4. API Endpoint Specifications

### 4.1 POST /v1/chat/completions

**Supported Request Parameters:**

| Parameter | Type | Required | Notes |
|-----------|------|----------|-------|
| `messages` | array | ✓ | User, assistant, system, developer, tool messages |
| `model` | string | ✓ | Currently simulates all models (gpt-4o, gpt-4, gpt-3.5-turbo) |
| `stream` | boolean | | Default: false. Enables SSE streaming |
| `stream_options` | object | | `{include_usage: bool}` |
| `tools` | array | | Function/custom tool definitions |
| `tool_choice` | string/object | | "auto", "required", "none", or specific tool |
| `temperature` | number | | 0-2 (affects text randomness) |
| `max_tokens` | integer | | Completion token limit |
| `max_completion_tokens` | integer | | New parameter (GPT-4o) |
| `top_p` | number | | Nucleus sampling (0-1) |
| `frequency_penalty` | number | | -2 to 2 |
| `presence_penalty` | number | | -2 to 2 |
| `n` | integer | | Number of choices (default: 1) |
| `response_format` | object | | `{type: "json_schema", ...}` for structured outputs |
| `logprobs` | boolean | | Include log probabilities |
| `top_logprobs` | integer | | Number of top log probs to return |
| `seed` | integer | | For reproducible outputs |
| `modalities` | array | | ["text"], ["text", "audio"] |
| `audio` | object | | Audio output config `{voice, format}` |
| `parallel_tool_calls` | boolean | | Allow multiple tool calls in parallel |

**Supported Messages:**

```go
// User message
{
    "role": "user",
    "content": "What is the weather?"
}

// System message
{
    "role": "system",
    "content": "You are a helpful assistant."
}

// Developer message (GPT-4o+)
{
    "role": "developer",
    "content": "Important system instructions..."
}

// Assistant message with tool calls
{
    "role": "assistant",
    "content": "I'll check that for you.",
    "tool_calls": [
        {
            "id": "call_12345",
            "type": "function",
            "function": {
                "name": "get_weather",
                "arguments": "{\"location\": \"San Francisco\"}"
            }
        }
    ]
}

// Tool response message
{
    "role": "tool",
    "tool_call_id": "call_12345",
    "content": "Sunny, 72°F"
}
```

**Response Format (Non-streaming):**

```json
{
    "id": "chatcmpl-8vYlDfLvNRBBM0HNZhgKVVLq8mfSr",
    "object": "chat.completion",
    "created": 1699564800,
    "model": "gpt-4o",
    "choices": [
        {
            "index": 0,
            "message": {
                "role": "assistant",
                "content": "The weather in San Francisco is sunny and 72 degrees Fahrenheit.",
                "tool_calls": null
            },
            "finish_reason": "stop",
            "logprobs": null
        }
    ],
    "usage": {
        "prompt_tokens": 15,
        "completion_tokens": 25,
        "total_tokens": 40
    },
    "system_fingerprint": "fp_12345"
}
```

**Response Format (Streaming):**

```
data: {"id":"chatcmpl-8vYlDfLvNRBBM0HNZhgKVVLq8mfSr","object":"chat.completion.chunk","created":1699564800,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"The"},"finish_reason":null}]}

data: {"id":"chatcmpl-8vYlDfLvNRBBM0HNZhgKVVLq8mfSr","object":"chat.completion.chunk","created":1699564800,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" weather"},"finish_reason":null}]}

...

data: {"id":"chatcmpl-8vYlDfLvNRBBM0HNZhgKVVLq8mfSr","object":"chat.completion.chunk","created":1699564800,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":15,"completion_tokens":25,"total_tokens":40}}

data: [DONE]
```

---

## 5. Tool Calling Implementation

### 5.1 Function Call Generation Strategy

**Context-Aware Approach:**
1. Analyze user message for keywords matching tool descriptions
2. Match tool names/descriptions to identified topics
3. Generate plausible JSON arguments matching tool schema
4. Support parallel tool calls (OpenAI 2024 feature)

**Example:**

```go
// User provides tools
tools := []Tool{
    {
        Name: "get_weather",
        Description: "Get weather for a location",
        Parameters: {
            Type: "object",
            Properties: {
                "location": {"type": "string"},
                "units": {"type": "string", "enum": ["celsius", "fahrenheit"]},
            },
            Required: ["location"],
        },
    },
    {
        Name: "search_web",
        Description: "Search the web for information",
        Parameters: {
            Type: "object",
            Properties: {
                "query": {"type": "string"},
            },
            Required: ["query"],
        },
    },
}

// User message: "What's the weather in San Francisco and Paris?"
// Generated response:
{
    "role": "assistant",
    "content": "I'll check the weather in both cities for you.",
    "tool_calls": [
        {
            "id": "call_123",
            "type": "function",
            "function": {
                "name": "get_weather",
                "arguments": "{\"location\": \"San Francisco\", \"units\": \"fahrenheit\"}"
            }
        },
        {
            "id": "call_124",
            "type": "function",
            "function": {
                "name": "get_weather",
                "arguments": "{\"location\": \"Paris\", \"units\": \"celsius\"}"
            }
        }
    ]
}
```

### 5.2 Structured Output Support

Generate responses adhering to JSON Schema constraints:

```go
// Request
{
    "model": "gpt-4o",
    "messages": [...],
    "response_format": {
        "type": "json_schema",
        "json_schema": {
            "name": "Person",
            "schema": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "age": {"type": "integer"},
                    "email": {"type": "string", "format": "email"},
                    "hobbies": {
                        "type": "array",
                        "items": {"type": "string"}
                    }
                },
                "required": ["name", "age", "email"]
            }
        }
    }
}

// Generated Response
{
    "name": "John Smith",
    "age": 32,
    "email": "john.smith@example.com",
    "hobbies": ["reading", "hiking", "photography"]
}
```

**Implementation:**
```go
func (g *ToolCallGenerator) GenerateStructuredOutput(schema JSONSchema) (string, error) {
    // 1. Parse JSON schema
    // 2. Generate values for each required property
    // 3. Respect type constraints (string, number, boolean, array, object)
    // 4. Validate generated object against schema
    // 5. Return as JSON string
    
    result := make(map[string]interface{})
    for _, prop := range schema.Required {
        propDef := schema.Properties[prop]
        result[prop] = g.generateValue(propDef)
    }
    
    jsonBytes, _ := json.Marshal(result)
    return string(jsonBytes), nil
}
```

---

## 6. Streaming Specification

### 6.1 Server-Sent Events (SSE) Format

**Headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
Transfer-Encoding: chunked
```

**Chunk Structure:**
```
data: <json-object>\n\n
```

**Example Stream:**
```
data: {"id":"...","object":"chat.completion.chunk","created":1699564800,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"...","object":"chat.completion.chunk","created":1699564800,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":null}]}

data: {"id":"...","object":"chat.completion.chunk","created":1699564800,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]
```

### 6.2 Tool Call Streaming

When `tool_calls` are triggered, stream tool call data:

```
data: {"id":"...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"name":"search_web"}}]},"finish_reason":null}]}

data: {"id":"...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"query\":"}}]},"finish_reason":null}]}

data: {"id":"...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"weather forecast\"}"}}]},"finish_reason":"tool_calls"}]}

data: [DONE]
```

---

## 7. Technology Stack & Dependencies

### 7.1 Core Dependencies

| Dependency | Version | Purpose | Alternative |
|-----------|---------|---------|-------------|
| `net/http` | Go stdlib | HTTP server | gin-gonic, echo, fiber |
| `encoding/json` | Go stdlib | JSON serialization | jsoniter (faster) |
| `github.com/google/uuid` | Latest | UUID generation | github.com/segmentio/lds |
| `github.com/stretchr/testify` | v1.8+ | Testing assertions | Testing package |

### 7.2 Optional Dependencies (Recommended)

- **`gin-gonic/gin`**: If building production API with middleware
- **`github.com/openai/openai-go/v3`**: For type reference (don't import, just copy types)
- **`github.com/stretchr/testify/mock`**: For unit testing

### 7.3 No External LLM Dependencies ✓
- ❌ Do NOT import: `openai-go` client (for calling real API)
- ❌ Do NOT depend on: LangChain, LlamaIndex, or other LLM frameworks
- ❌ Do NOT require: API keys, credentials, or external services

---

## 8. Project Structure

```
openai-api-simulator/
├── adr/
│   └── 0001-openai-api-simulator-go-implementation.md (this file)
├── cmd/
│   └── simulator/
│       └── main.go                    # Server entry point
├── pkg/
│   ├── api/
│   │   ├── handler.go                # HTTP request handlers
│   │   ├── server.go                 # HTTP server setup
│   │   └── validator.go              # Request validation
│   ├── generator/
│   │   ├── text_generator.go         # Coherent sentence generation
│   │   ├── tool_generator.go         # Tool call generation
│   │   └── response_builder.go       # Response assembly
│   ├── models/
│   │   ├── chat_completion.go        # OpenAI response types
│   │   ├── message.go                # Message types
│   │   ├── tool.go                   # Tool definitions
│   │   └── usage.go                  # Token usage
│   ├── streaming/
│   │   ├── handler.go                # SSE streaming logic
│   │   └── encoder.go                # SSE encoding
│   └── utils/
│       ├── tokenizer.go              # Token estimation
│       ├── id_generator.go           # ID creation
│       └── wordbank.go               # Sentence word banks
├── internal/
│   ├── config/
│   │   └── config.go                 # Configuration management
│   └── logging/
│       └── logger.go                 # Logging setup
├── test/
│   ├── integration_test.go           # Full endpoint tests
│   ├── streaming_test.go             # Streaming validation
│   ├── tool_calling_test.go          # Tool call generation
│   └── fixtures/
│       ├── requests.json             # Example requests
│       └── responses.json            # Example responses
├── examples/
│   ├── basic_completion.go           # Basic usage example
│   ├── streaming_example.go          # Streaming example
│   └── tool_calling_example.go       # Tool call example
├── go.mod                            # Go module definition
├── go.sum                            # Dependency checksums
├── Dockerfile                        # Container configuration
├── docker-compose.yml                # Multi-container setup
├── Makefile                          # Build automation
└── README.md                         # Usage documentation
```

---

## 9. Implementation Priorities

### Phase 1: MVP (Week 1)
- [x] Data models (ChatCompletion, ChatCompletionChunk)
- [x] HTTP server with POST /v1/chat/completions
- [x] Basic text generation (coherent sentences)
- [x] Non-streaming responses
- [x] Token estimation
- [ ] Docker containerization

### Phase 2: Streaming & Tools (Week 2)
- [ ] SSE streaming implementation
- [ ] Tool call generation (random strategy)
- [ ] Tool call streaming (chunked JSON)
- [ ] Parallel tool call support
- [ ] Integration tests

### Phase 3: Advanced Features (Week 3)
- [ ] Structured outputs (JSON Schema validation)
- [ ] Context-aware tool calling (keyword matching)
- [ ] Audio response simulation (base64 encoding)
- [ ] Logprobs calculation
- [ ] Deterministic mode (seed-based generation)
- [ ] Performance benchmarks

### Phase 4: Production Hardening (Week 4)
- [ ] Load testing (1000+ req/sec)
- [ ] Error handling & recovery
- [ ] Configuration management
- [ ] Comprehensive logging
- [ ] Health check endpoint
- [ ] Metrics collection (Prometheus compatible)
- [ ] Documentation & examples

---

## 10. Testing Strategy

### 10.1 Unit Tests

**Test Coverage: 90%+**

```go
// pkg/generator/text_generator_test.go
func TestGenerateText_Coherence(t *testing.T) {
    gen := NewCoherentTextGenerator()
    
    // Test: Output is coherent sentences
    text := gen.GenerateText(context.Background(), 100, 500)
    assert.NotEmpty(t, text)
    assert.Greater(t, len(text), 100)
    assert.Less(t, len(text), 500)
    
    // Test: Contains proper grammar
    assert.Regexp(t, `[.!?]\s`, text) // Has punctuation
}

func TestGenerateText_Variation(t *testing.T) {
    gen := NewCoherentTextGenerator()
    
    // Test: Different calls produce different output
    text1 := gen.GenerateText(context.Background(), 100, 200)
    text2 := gen.GenerateText(context.Background(), 100, 200)
    assert.NotEqual(t, text1, text2)
}

func TestGenerateText_Determinism(t *testing.T) {
    // Test: Same seed produces same output
    gen1 := NewCoherentTextGeneratorWithSeed(12345)
    gen2 := NewCoherentTextGeneratorWithSeed(12345)
    
    text1 := gen1.GenerateText(context.Background(), 100, 200)
    text2 := gen2.GenerateText(context.Background(), 100, 200)
    assert.Equal(t, text1, text2)
}

// pkg/generator/tool_generator_test.go
func TestGenerateToolCalls_ValidJSON(t *testing.T) {
    gen := NewToolCallGenerator()
    tools := []ToolDefinition{
        {
            Name: "search",
            Parameters: JSONSchema{
                Type: "object",
                Properties: map[string]PropertyDef{
                    "query": {Type: "string"},
                },
                Required: []string{"query"},
            },
        },
    }
    
    calls, err := gen.GenerateToolCalls(context.Background(), tools, "find weather", StrategyRandom)
    
    assert.NoError(t, err)
    assert.Len(t, calls, 1)
    
    // Validate JSON arguments
    var args map[string]interface{}
    err = json.Unmarshal([]byte(calls[0].Function.Arguments), &args)
    assert.NoError(t, err)
    assert.Contains(t, args, "query")
}
```

### 10.2 Integration Tests

```go
// test/integration_test.go
func TestChatCompletionEndpoint_NonStreaming(t *testing.T) {
    server := setupTestServer()
    defer server.Close()
    
    payload := ChatCompletionNewParams{
        Model: "gpt-4o",
        Messages: []ChatCompletionMessage{
            {Role: "user", Content: "Hello"},
        },
    }
    
    body, _ := json.Marshal(payload)
    resp, _ := http.Post(
        server.URL+"/v1/chat/completions",
        "application/json",
        bytes.NewBuffer(body),
    )
    
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    var result ChatCompletion
    json.NewDecoder(resp.Body).Decode(&result)
    
    assert.Equal(t, "chat.completion", result.Object)
    assert.Equal(t, "gpt-4o", result.Model)
    assert.Len(t, result.Choices, 1)
    assert.NotEmpty(t, result.Choices[0].Message.Content)
}

func TestChatCompletionEndpoint_Streaming(t *testing.T) {
    server := setupTestServer()
    defer server.Close()
    
    payload := ChatCompletionNewParams{
        Model:  "gpt-4o",
        Stream: true,
        Messages: []ChatCompletionMessage{
            {Role: "user", Content: "Hello"},
        },
    }
    
    body, _ := json.Marshal(payload)
    resp, _ := http.Post(
        server.URL+"/v1/chat/completions",
        "application/json",
        bytes.NewBuffer(body),
    )
    
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
    
    // Parse SSE chunks
    scanner := bufio.NewScanner(resp.Body)
    chunks := 0
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: {") {
            chunks++
            var chunk ChatCompletionChunk
            json.Unmarshal([]byte(line[6:]), &chunk)
            assert.Equal(t, "chat.completion.chunk", chunk.Object)
        }
    }
    
    assert.Greater(t, chunks, 0)
}
```

### 10.3 Compliance Tests

```go
// test/openai_compliance_test.go
func TestResponseSchema_Compliance(t *testing.T) {
    // Validate response matches OpenAI API schema
    
    // Reference: github.com/openai/openai-go/blob/main/chatcompletion.go
    
    // Required fields in ChatCompletion
    result := &ChatCompletion{}
    assert.NotNil(t, result.ID)
    assert.NotNil(t, result.Object)
    assert.NotNil(t, result.Created)
    assert.NotNil(t, result.Model)
    assert.NotNil(t, result.Choices)
    assert.NotNil(t, result.Usage)
}

func TestToolCallSchema_Compliance(t *testing.T) {
    // Validate tool call matches OpenAI spec
    call := ChatCompletionMessageToolCall{
        ID:   "call_xyz",
        Type: "function",
        Function: ChatCompletionToolCallFunction{
            Name:      "get_weather",
            Arguments: `{"location": "San Francisco"}`,
        },
    }
    
    // Marshal and validate JSON
    data, _ := json.Marshal(call)
    
    // Should be deserializable
    var unmarshaled ChatCompletionMessageToolCall
    err := json.Unmarshal(data, &unmarshaled)
    assert.NoError(t, err)
}
```

---

## 11. Performance Targets

### 11.1 Latency

| Metric | Target | Notes |
|--------|--------|-------|
| Non-streaming response | < 100ms | Text generation + JSON marshaling |
| Stream first chunk | < 50ms | Initial SSE chunk |
| Chunk latency | 5-20ms | Between SSE chunks (configurable) |
| Tool call generation | < 75ms | Schema validation + JSON generation |

### 11.2 Throughput

| Metric | Target |
|--------|--------|
| Requests/second | 1,000+ per core |
| Concurrent connections | 10,000+ (with proper tuning) |
| Memory per request | < 2MB |

### 11.3 Text Generation

| Metric | Target |
|--------|--------|
| Words generated/second | > 10,000 |
| Variability | No repetition within 100 requests |
| Coherence score | Native speaker readable (manual review) |

---

## 12. Security Considerations

### 12.1 Input Validation

- ✓ Validate JSON schema for all requests
- ✓ Limit message size (< 1MB per message)
- ✓ Validate tool parameter counts
- ✓ Escape user input in responses (if echoed)
- ✓ Rate limiting (optional, configurable)

### 12.2 Output Safety

- ✓ No sensitive data in responses
- ✓ No LLM prompts exposed
- ✓ Deterministic responses (no edge cases)
- ✓ No external service calls possible

### 12.3 API Keys & Authentication

**Design Decision: No API key requirement in MVP**
- Simulator is for testing/development only
- Deploy behind authentication layer in production
- Can add optional Bearer token validation

```go
// Optional middleware for API key validation
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // token := r.Header.Get("Authorization")
        // Validate token if needed
        next.ServeHTTP(w, r)
    })
}
```

---

## 13. Observability

### 13.1 Logging

```go
// Every request should log:
log.Info("chat.completion.requested",
    "model", req.Model,
    "stream", req.Stream,
    "message_count", len(req.Messages),
    "has_tools", len(req.Tools) > 0,
)
```

### 13.2 Metrics (Prometheus)

```
openai_simulator_requests_total{method="create_completion", status="success"} 1234
openai_simulator_request_duration_seconds{method="create_completion"} 0.045
openai_simulator_streaming_chunks_total 5678
openai_simulator_tool_calls_generated_total 234
```

### 13.3 Tracing

Optional OpenTelemetry integration:
```go
tracer.Start(ctx, "generate_text")
defer span.End()
```

---

## 14. Example Usage

### 14.1 Running the Simulator

```bash
# Build
go build -o simulator ./cmd/simulator

# Run with defaults (localhost:8080)
./simulator

# Run with custom config
./simulator --port 3000 --addr 127.0.0.1
```

### 14.2 Using with OpenAI Go SDK

```go
import "github.com/openai/openai-go/v3"

client := openai.NewClient(
    option.WithAPIKey("fake-key-for-testing"),
    option.WithBaseURL("http://localhost:8080/v1"),
)

resp, _ := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("What is Go?"),
    },
    Model: openai.ChatModelGPT4o,
})

println(resp.Choices[0].Message.Content)
```

### 14.3 Streaming Example

```go
stream := client.Chat.Completions.NewStreaming(context.Background(), openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Explain AI"),
    },
    Model:  openai.ChatModelGPT4o,
    Stream: openai.Bool(true),
})

for stream.Next() {
    chunk := stream.Current()
    if len(chunk.Choices) > 0 {
        print(chunk.Choices[0].Delta.Content)
    }
}
```

---

## 15. Known Limitations & Future Work

### 15.1 Current Limitations

1. **No actual reasoning**: Generated text is random, not contextual
2. **No memory**: Each request is independent (stateless)
3. **No vision**: Image/file inputs accepted but not analyzed
4. **No actual audio**: Audio field is simulated (base64 placeholder)
5. **Single model simulation**: All models behave identically
6. **No fine-tuning**: Responses don't improve with feedback

### 15.2 Future Enhancements

- [ ] Context-aware responses (keyword matching in user message)
- [ ] Conversation memory (track messages across requests)
- [ ] Multi-turn tool calling (tool responses → next request)
- [ ] Different "personalities" (model variations)
- [ ] Custom response templates (per deployment)
- [ ] Actual vision processing (local CV models)
- [ ] Real audio generation (text-to-speech simulation)

---

## 16. References & Resources

### 16.1 OpenAI Official Documentation

- **API Reference**: https://github.com/openai/openai-go
- **Chat Completions Guide**: [OpenAI Docs](https://platform.openai.com/docs/api-reference/chat)
- **Function Calling**: [OpenAI Docs](https://platform.openai.com/docs/guides/function-calling)
- **Streaming**: [OpenAI Docs](https://platform.openai.com/docs/api-reference/chat/create#chat-create-stream)
- **Structured Outputs**: [OpenAI Docs](https://platform.openai.com/docs/guides/structured-outputs)

### 16.2 Go SDK Reference

- **OpenAI Go Library**: https://github.com/openai/openai-go
- **Type Definitions**: https://github.com/openai/openai-go/blob/main/chatcompletion.go
- **Streaming Implementation**: https://github.com/openai/openai-go/blob/main/packages/ssestream/stream.go

### 16.3 Server-Sent Events (SSE)

- **MDN SSE Spec**: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events
- **Format**: `data: {json}\n\n`

### 16.4 Go Best Practices

- **Error Handling**: https://go.dev/blog/error-handling-and-go
- **Concurrency**: https://go.dev/tour/concurrency/1
- **Testing**: https://golang.org/doc/effective_go#testing

---

## 17. Decision Rationale

### Why Go?

| Aspect | Rationale |
|--------|-----------|
| **Performance** | Compiled, fast execution (< 100ms target) |
| **Concurrency** | Goroutines handle 10K+ concurrent streams |
| **Simplicity** | Minimal dependencies, easy to deploy |
| **Stdlib** | `net/http`, `encoding/json` sufficient |
| **Containers** | Small Docker images, fast startup |

### Why Not Python?

- Slower (100-300ms for equivalent code)
- GIL limits concurrent request handling
- More memory overhead
- Larger Docker images

### Why Coherent Text Over Random Words?

- Random words are obviously fake (fails Turing test)
- Coherent sentences appear realistic to tests
- Matches user expectations for "realistic" simulation
- Still deterministic with seed support

### Why SSE Over WebSocket?

- OpenAI uses SSE for streaming
- Simpler, unidirectional (client only receives)
- Works with standard HTTP proxies/load balancers
- Native browser support (EventSource API)

---

## 18. Approval & Sign-Off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Tech Lead | | | |
| Security | | | |
| Product Owner | | | |

---

## 19. Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-11-17 | Initial ADR |

---

**Document Classification**: Internal Engineering  
**Revision Date**: November 17, 2025  
**Next Review**: January 17, 2026 (Post-MVP)
