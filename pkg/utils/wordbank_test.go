package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSentenceAndParagraph(t *testing.T) {
	wb := NewWordBank()

	// Ensure a sentence is returned
	s := wb.GenerateSentence()
	require.NotEmpty(t, s)

	// Ensure paragraph returns multiple sentences
	p := wb.GenerateParagraph(4)
	require.NotEmpty(t, p)
	require.Greater(t, len(p), len(s))

	// Ensure GenerateSentence variability over multiple calls
	s2 := wb.GenerateSentence()
	require.NotEqual(t, s, s2)
}
