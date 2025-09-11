package services

import (
	"strings"
	"unicode"
)

// TextAnalyzer provides text analysis capabilities for the domain
// This is a domain service that encapsulates text processing logic
type TextAnalyzer interface {
	// ExtractKeywords extracts meaningful keywords from text
	ExtractKeywords(text string) []string
	
	// TokenizeWords breaks text into a set of unique lowercase words
	TokenizeWords(text string) map[string]bool
	
	// ExtractSignificantWords gets words above a certain length threshold
	ExtractSignificantWords(text string, minLength int) []string
}

// DefaultTextAnalyzer provides a default implementation of TextAnalyzer
type DefaultTextAnalyzer struct {
	stopWords map[string]bool
}

// NewDefaultTextAnalyzer creates a new text analyzer with common English stop words
func NewDefaultTextAnalyzer() *DefaultTextAnalyzer {
	return &DefaultTextAnalyzer{
		stopWords: getDefaultStopWords(),
	}
}

// ExtractKeywords extracts meaningful keywords from text
func (ta *DefaultTextAnalyzer) ExtractKeywords(text string) []string {
	words := ta.TokenizeWords(text)
	keywords := make([]string, 0)
	
	for word := range words {
		// Skip stop words and very short words
		if !ta.stopWords[word] && len(word) > 2 {
			keywords = append(keywords, word)
		}
	}
	
	return keywords
}

// TokenizeWords breaks text into a set of unique lowercase words
func (ta *DefaultTextAnalyzer) TokenizeWords(text string) map[string]bool {
	words := make(map[string]bool)
	text = strings.ToLower(text)
	
	// Simple tokenization - split on non-letter characters
	var currentWord strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentWord.WriteRune(r)
		} else if currentWord.Len() > 0 {
			word := currentWord.String()
			if len(word) > 1 { // Skip single characters
				words[word] = true
			}
			currentWord.Reset()
		}
	}
	
	// Don't forget the last word
	if currentWord.Len() > 0 {
		word := currentWord.String()
		if len(word) > 1 {
			words[word] = true
		}
	}
	
	return words
}

// ExtractSignificantWords gets words above a certain length threshold
func (ta *DefaultTextAnalyzer) ExtractSignificantWords(text string, minLength int) []string {
	words := ta.TokenizeWords(text)
	significant := make([]string, 0)
	
	for word := range words {
		if len(word) >= minLength && !ta.stopWords[word] {
			significant = append(significant, word)
		}
	}
	
	return significant
}

// getDefaultStopWords returns a set of common English stop words
func getDefaultStopWords() map[string]bool {
	stopWords := map[string]bool{
		"the": true, "be": true, "to": true, "of": true, "and": true,
		"a": true, "in": true, "that": true, "have": true, "i": true,
		"it": true, "for": true, "not": true, "on": true, "with": true,
		"he": true, "as": true, "you": true, "do": true, "at": true,
		"this": true, "but": true, "his": true, "by": true, "from": true,
		"they": true, "we": true, "say": true, "her": true, "she": true,
		"or": true, "an": true, "will": true, "my": true, "one": true,
		"all": true, "would": true, "there": true, "their": true, "what": true,
		"so": true, "up": true, "out": true, "if": true, "about": true,
		"who": true, "get": true, "which": true, "go": true, "me": true,
		"when": true, "make": true, "can": true, "like": true, "time": true,
		"no": true, "just": true, "him": true, "know": true, "take": true,
		"people": true, "into": true, "year": true, "your": true, "good": true,
		"some": true, "could": true, "them": true, "see": true, "other": true,
		"than": true, "then": true, "now": true, "look": true, "only": true,
		"come": true, "its": true, "over": true, "think": true, "also": true,
		"back": true, "after": true, "use": true, "two": true, "how": true,
		"our": true, "work": true, "first": true, "well": true, "way": true,
		"even": true, "new": true, "want": true, "because": true, "any": true,
		"these": true, "give": true, "day": true, "most": true, "us": true,
		"is": true, "was": true, "are": true, "been": true, "has": true,
		"had": true, "were": true, "said": true, "did": true, "having": true,
		"may": true, "am": true, "should": true, "too": true, "very": true,
	}
	return stopWords
}