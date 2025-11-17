package generator

import (
	"context"
	"testing"

	"github.com/openai/openai-api-simulator/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateToolCalls(t *testing.T) {
	g := NewToolCallGenerator()
	tools := []ToolDefinition{
		{Function: models.FunctionDefinition{Name: "do_a"}, Type: "function"},
		{Function: models.FunctionDefinition{Name: "do_b"}, Type: "function"},
	}

	calls, err := g.GenerateToolCalls(context.Background(), tools, StrategyRandom)
	require.NoError(t, err)
	require.True(t, len(calls) >= 1)
}

func TestGenerateStructuredOutput(t *testing.T) {
	g := NewToolCallGenerator()
	schema := models.JSONSchema{
		Type: "object",
		Properties: map[string]models.PropertyDef{
			"name":  {Type: "string"},
			"age":   {Type: "integer"},
			"email": {Type: "string"},
		},
		Required: []string{"name", "email"},
	}

	out, err := g.GenerateStructuredOutput(schema)
	require.NoError(t, err)
	require.Contains(t, out, "name")
	require.Contains(t, out, "email")
}
