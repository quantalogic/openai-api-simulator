package generator

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/openai/openai-api-simulator/pkg/utils"
)

// TextGenerator defines the interface for generating text
type TextGenerator interface {
	GenerateText(ctx context.Context, minLength, maxLength int) string
	GenerateChunk(ctx context.Context) string
}

// CoherentTextGenerator generates coherent, variable-length English text
type CoherentTextGenerator struct {
	wordBank *utils.WordBank
	rand     *rand.Rand
	seed     int64
}

// NewCoherentTextGenerator creates a new text generator
func NewCoherentTextGenerator() *CoherentTextGenerator {
	return &CoherentTextGenerator{
		wordBank: utils.NewWordBank(),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		seed:     0,
	}
}

// NewCoherentTextGeneratorWithSeed creates a text generator with a specific seed
func NewCoherentTextGeneratorWithSeed(seed int64) *CoherentTextGenerator {
	return &CoherentTextGenerator{
		wordBank: utils.NewWordBank(),
		rand:     rand.New(rand.NewSource(seed)),
		seed:     seed,
	}
}

// GenerateText generates variable-length coherent English text
func (g *CoherentTextGenerator) GenerateText(ctx context.Context, minLength, maxLength int) string {
	// We produce 1-3 paragraphs, each with variable sentence counts.
	// Paragraphs are joined with double newlines to mimic human-like responses.
	paragraphs := []string{}

	// Choose number of paragraphs: mostly 1, sometimes 2 or 3.
	paragraphProb := g.rand.Intn(10) // 0..9
	numParagraphs := 1
	if paragraphProb < 2 {
		numParagraphs = 3
	} else if paragraphProb < 5 {
		numParagraphs = 2
	}

	totalLen := 0
	for p := 0; p < numParagraphs; p++ {
		// Determine paragraph length profile: short, medium, or long
		r := g.rand.Float32()
		var minSent, maxSent int
		switch {
		case r < 0.25:
			// Very short paragraphs (1-2 sentences)
			minSent, maxSent = 1, 2
		case r < 0.7:
			// Medium paragraphs (2-5 sentences)
			minSent, maxSent = 2, 5
		default:
			// Long paragraphs (5-10 sentences)
			minSent, maxSent = 5, 10
		}

		sentences := g.rand.Intn(maxSent-minSent+1) + minSent
		paragraph := g.wordBank.GenerateParagraph(sentences)
		paragraphs = append(paragraphs, paragraph)
		totalLen += len(paragraph)

		if totalLen >= maxLength { // stop if we've already reached target
			break
		}
	}

	text := strings.Join(paragraphs, "\n\n")

	// If text is too short, keep appending short paragraphs until minLength is reached
	attempts := 0
	for len(text) < minLength && attempts < 5 {
		paragraph := g.wordBank.GenerateParagraph(g.rand.Intn(3) + 1)
		text += "\n\n" + paragraph
		attempts++
	}

	// Truncate to within maximum length while preserving last sentence boundary
	if len(text) > maxLength {
		short := text[:maxLength]
		lastPeriod := strings.LastIndex(short, ".")
		if lastPeriod > 0 {
			short = short[:lastPeriod+1]
		}
		text = short
	}

	return strings.TrimSpace(text)
}

// GenerateChunk generates a single streaming chunk (1-3 words)
func (g *CoherentTextGenerator) GenerateChunk(ctx context.Context) string {
	numWords := g.rand.Intn(3) + 1
	words := make([]string, numWords)

	for i := 0; i < numWords; i++ {
		// Use a simple word selection
		word := utils.RandomString(g.wordBank.Nouns)
		if i == 0 && g.rand.Float32() > 0.5 {
			word = utils.RandomString(g.wordBank.Adjectives)
		}
		words[i] = word
	}

	return strings.Join(words, " ")
}
