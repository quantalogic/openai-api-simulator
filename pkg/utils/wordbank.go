package utils

import "strings"

// WordBank contains lists of words for generating coherent text
type WordBank struct {
	Nouns        []string
	Verbs        []string
	Adjectives   []string
	Adverbs      []string
	Transitions  []string
	Subjects     []string
	Objects      []string
	Places       []string
	Names        []string
	Pronouns     []string
	Determiners  []string
	Conjunctions []string
}

// NewWordBank creates a new word bank with common words
func NewWordBank() *WordBank {
	return &WordBank{
		Nouns: []string{
			"system", "approach", "solution", "framework", "architecture", "pattern",
			"design", "implementation", "strategy", "process", "method", "technique",
			"technology", "platform", "service", "module", "component", "feature",
			"application", "software", "code", "data", "information", "structure",
		},
		Verbs: []string{
			"implement", "develop", "create", "design", "build", "establish",
			"provide", "enable", "support", "enhance", "improve", "optimize",
			"simplify", "streamline", "integrate", "connect", "combine", "merge",
			"handle", "manage", "process", "execute", "perform", "achieve",
		},
		Adjectives: []string{
			"robust", "efficient", "scalable", "reliable", "secure", "flexible",
			"modular", "clean", "elegant", "sophisticated", "advanced", "modern",
			"innovative", "comprehensive", "complete", "effective", "powerful",
		},
		Adverbs: []string{
			"successfully", "effectively", "efficiently", "carefully", "properly",
			"thoroughly", "systematically", "methodically", "comprehensively", "strategically",
			"seamlessly", "transparently", "reliably", "consistently", "dynamically",
		},
		Transitions: []string{
			"Furthermore", "Moreover", "Additionally", "In addition", "Similarly",
			"However", "Nevertheless", "On the other hand", "Conversely", "In contrast",
			"Therefore", "Thus", "Consequently", "As a result", "Subsequently",
		},
		Subjects: []string{
			"The system", "This approach", "The framework", "Our solution", "The architecture",
			"The implementation", "This design", "The technology", "Our platform",
		},
		Objects: []string{
			"efficiency", "reliability", "scalability", "security", "flexibility",
			"simplicity", "clarity", "maintainability", "performance", "accuracy",
		},
		Places: []string{
			"San Francisco", "New York", "London", "Berlin", "Tokyo",
			"Sydney", "Paris", "Austin", "Seattle", "Toronto",
		},
		Names: []string{
			"Alice", "Bob", "Carol", "Dave", "Eve",
			"Grace", "Heidi", "Mallory", "Trent", "Peggy",
		},
		Pronouns: []string{
			"it", "they", "we", "you", "he", "she", "one",
		},
		Determiners: []string{
			"the", "a", "an", "this", "that", "these", "those",
		},
		Conjunctions: []string{
			"and", "but", "or", "so", "because", "since", "although",
		},
	}
}

// GetRandomSentenceTemplate returns a random sentence template
func (wb *WordBank) GetRandomSentenceTemplate() string {
	templates := []string{
		"{subject} {verb} {adj} {obj}.",
		"{trans}, {subject} {verb} {adj} {obj}.",
		"The {adj} {noun} {verb} {adv}.",
		"This {noun} {verb} {adj} {noun}.",
		"In this {noun}, we {verb} {adj} {noun}.",
		"{subject} is {adj} when {pronoun} {verb} {obj}.",
		"{trans} {subject} and {subject} {verb} {obj}.",
		"{subject} in {place} {verb} {adj} {obj}.",
	}
	return RandomString(templates)
}

// GenerateSentence generates a coherent sentence using the word bank
func (wb *WordBank) GenerateSentence() string {
	template := wb.GetRandomSentenceTemplate()

	replacements := map[string]string{
		"{noun}":    RandomString(wb.Nouns),
		"{verb}":    RandomString(wb.Verbs),
		"{adj}":     RandomString(wb.Adjectives),
		"{adv}":     RandomString(wb.Adverbs),
		"{trans}":   RandomString(wb.Transitions),
		"{subject}": RandomString(wb.Subjects),
		"{obj}":     RandomString(wb.Objects),
		"{place}":   RandomString(wb.Places),
		"{name}":    RandomString(wb.Names),
		"{pronoun}": RandomString(wb.Pronouns),
	}

	result := template
	for placeholder, word := range replacements {
		result = strings.ReplaceAll(result, placeholder, word)
	}

	return result
}

// GenerateParagraph returns multiple sentences joined together for more realistically-lengthy outputs.
func (wb *WordBank) GenerateParagraph(sentences int) string {
	if sentences <= 0 {
		sentences = 3
	}

	var out []string
	for i := 0; i < sentences; i++ {
		out = append(out, wb.GenerateSentence())
	}
	return strings.Join(out, " ")
}
