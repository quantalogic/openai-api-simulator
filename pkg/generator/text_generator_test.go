package generator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateTextVariesLength(t *testing.T) {
	// Use multiple seeds to ensure variability
	for seed := int64(1); seed < 10; seed++ {
		g := NewCoherentTextGeneratorWithSeed(seed)
		text := g.GenerateText(context.Background(), 30, 300)
		length := len(text)
		require.GreaterOrEqual(t, length, 30)
		require.LessOrEqual(t, length, 300)
	}
}

func TestGenerateTextProducesParagraphs(t *testing.T) {
	// Seeds that are likely to produce multiple paragraphs
	seeds := []int64{2, 3, 5, 7, 11}
	foundMulti := false
	for _, s := range seeds {
		g := NewCoherentTextGeneratorWithSeed(s)
		text := g.GenerateText(context.Background(), 60, 900)
		if strings.Contains(text, "\n\n") {
			foundMulti = true
			break
		}
	}
	require.True(t, foundMulti, "expected at least one generated text to contain multiple paragraphs")
}
