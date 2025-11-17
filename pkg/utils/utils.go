package utils

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// IDGenerator generates unique IDs
type IDGenerator struct {
	rand *rand.Rand
}

// NewIDGenerator creates a new ID generator
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateID generates a unique ID
func (g *IDGenerator) GenerateID() string {
	return "chatcmpl-" + uuid.New().String()[:12]
}

// GenerateToolCallID generates a tool call ID
func (g *IDGenerator) GenerateToolCallID() string {
	return "call_" + uuid.New().String()[:24]
}

// EstimateTokens estimates the number of tokens in a string
func EstimateTokens(text string) int64 {
	if text == "" {
		return 1
	}
	tokens := len(text) / 4
	if tokens < 1 {
		tokens = 1
	}
	return int64(tokens)
}

// TokenizeText tokenizes text into words
func TokenizeText(text string) []string {
	return strings.Fields(text)
}

// RandomInt generates a random integer between min and max
func RandomInt(min, max int) int {
	if min > max {
		min, max = max, min
	}
	return rand.Intn(max-min+1) + min
}

// RandomString picks a random string from a slice
func RandomString(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	return slice[rand.Intn(len(slice))]
}

// ContainsKeyword checks if text contains any keywords
func ContainsKeyword(text string, keywords []string) bool {
	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// Pointer returns a pointer to a value
func Pointer[T any](v T) *T {
	return &v
}

// StringPointer returns a pointer to a string
func StringPointer(s string) *string {
	return &s
}

// Int64Pointer returns a pointer to an int64
func Int64Pointer(i int64) *int64 {
	return &i
}

// Float64Pointer returns a pointer to a float64
func Float64Pointer(f float64) *float64 {
	return &f
}

// BoolPointer returns a pointer to a bool
func BoolPointer(b bool) *bool {
	return &b
}

// JSONStringValue generates a random string value
func JSONStringValue(examples []string) string {
	if len(examples) > 0 && rand.Float64() < 0.7 {
		return RandomString(examples)
	}
	adjectives := []string{"important", "complex", "modern", "innovative"}
	nouns := []string{"solution", "approach", "system", "framework"}
	return fmt.Sprintf("%s %s", RandomString(adjectives), RandomString(nouns))
}

// JSONNumberValue generates a random number value
func JSONNumberValue() float64 {
	return rand.Float64() * 1000
}

// JSONBoolValue generates a random boolean value
func JSONBoolValue() bool {
	return rand.Float64() < 0.5
}
