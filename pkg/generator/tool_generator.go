package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/quantalogic/openai-api-simulator/pkg/models"
	"github.com/quantalogic/openai-api-simulator/pkg/utils"
)

// ToolCallStrategy controls how tools are invoked.
type ToolCallStrategy string

const (
	StrategySequence   ToolCallStrategy = "sequence"
	StrategyContextual ToolCallStrategy = "contextual"
	StrategyRandom     ToolCallStrategy = "random"
)

// ToolDefinition describes a tool we can simulate.
type ToolDefinition struct {
	Function models.FunctionDefinition
	Type     string
}

// ToolCallGenerator produces simulated function call output.
type ToolCallGenerator struct {
	rand  *rand.Rand
	idGen *utils.IDGenerator
}

// NewToolCallGenerator builds a generator ready to create calls.
func NewToolCallGenerator() *ToolCallGenerator {
	return &ToolCallGenerator{
		rand:  rand.New(rand.NewSource(time.Now().UnixNano())),
		idGen: utils.NewIDGenerator(),
	}
}

// GenerateToolCalls fabricates tool calls using the given strategy.
func (g *ToolCallGenerator) GenerateToolCalls(
	ctx context.Context,
	tools []ToolDefinition,
	strategy ToolCallStrategy,
) ([]models.ChatCompletionMessageToolCall, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	switch strategy {
	case StrategySequence:
		return g.sequenceCalls(tools), nil
	case StrategyContextual:
		return g.contextualCalls(tools), nil
	default:
		return g.randomCalls(tools), nil
	}
}

func (g *ToolCallGenerator) sequenceCalls(tools []ToolDefinition) []models.ChatCompletionMessageToolCall {
	var calls []models.ChatCompletionMessageToolCall
	for _, tool := range tools {
		calls = append(calls, g.generateCall(tool))
	}
	return calls
}

func (g *ToolCallGenerator) contextualCalls(tools []ToolDefinition) []models.ChatCompletionMessageToolCall {
	var calls []models.ChatCompletionMessageToolCall
	for _, tool := range tools {
		if g.rand.Float64() < 0.5 {
			calls = append(calls, g.generateCall(tool))
		}
	}
	return calls
}

func (g *ToolCallGenerator) randomCalls(tools []ToolDefinition) []models.ChatCompletionMessageToolCall {
	num := g.rand.Intn(len(tools)) + 1
	return g.sequenceCalls(shuffleTools(tools)[:num])
}

func shuffleTools(tools []ToolDefinition) []ToolDefinition {
	shuffled := append([]ToolDefinition(nil), tools...)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}

func (g *ToolCallGenerator) generateCall(tool ToolDefinition) models.ChatCompletionMessageToolCall {
	return models.ChatCompletionMessageToolCall{
		ID:   g.idGen.GenerateToolCallID(),
		Type: nonEmpty(tool.Type, "function"),
		Function: models.ChatCompletionMessageToolCallFunction{
			Name:      tool.Function.Name,
			Arguments: g.generateJSONArguments(tool.Function.Parameters),
		},
	}
}

func (g *ToolCallGenerator) generateJSONArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return "{}"
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return "{}"
	}

	result := make(map[string]interface{})
	for name := range properties {
		result[name] = g.fakeJSONValue()
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// GenerateStructuredOutput generates a JSON string that matches the given JSONSchema definition.
// This is used by the simulator to produce structured outputs for response_format: json_schema.
func (g *ToolCallGenerator) GenerateStructuredOutput(schema models.JSONSchema) (string, error) {
	// Only handle object schemas for MVP
	if schema.Type != "object" {
		// fallback to empty object
		return "{}", nil
	}

	// Build output map
	result := make(map[string]interface{})

	for name, prop := range schema.Properties {
		// Always include required fields, optionally include non-required fields randomly
		if contains(schema.Required, name) || g.rand.Float32() > 0.2 {
			result[name] = g.generateValue(prop)
		}
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal structured output: %w", err)
	}
	return string(out), nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// generateValue creates a fake value matching the property definition.
func (g *ToolCallGenerator) generateValue(prop models.PropertyDef) interface{} {
	switch prop.Type {
	case "string":
		if len(prop.Enum) > 0 {
			// choose one
			if v, ok := prop.Enum[0].(string); ok {
				return v
			}
		}
		return fmt.Sprintf("str-%d", g.rand.Intn(1000))
	case "number", "float":
		return g.rand.Float64() * 100
	case "integer":
		return g.rand.Intn(100)
	case "boolean":
		return g.rand.Intn(2) == 0
	case "array":
		// simple array of strings
		return []string{fmt.Sprintf("item%d", g.rand.Intn(10))}
	case "object":
		// nested objects: include minimal keys
		out := map[string]interface{}{}
		for k, p := range prop.Properties {
			out[k] = g.generateValue(p)
		}
		return out
	default:
		return fmt.Sprintf("val-%d", g.rand.Intn(1000))
	}
}

func (g *ToolCallGenerator) fakeJSONValue() interface{} {
	switch g.rand.Intn(3) {
	case 0:
		return fmt.Sprintf("value-%d", g.rand.Intn(1000))
	case 1:
		return g.rand.Float64() * 100
	default:
		return g.rand.Intn(2) == 0
	}
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
