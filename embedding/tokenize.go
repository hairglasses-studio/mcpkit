package embedding

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultMinTokenLength is the minimum token length used by DefaultTokenizer.
const DefaultMinTokenLength = 2

// Tokenizer converts free text into normalized sparse-vector terms.
type Tokenizer struct {
	// MinLength drops tokens shorter than this many runes. Values <= 0 use
	// DefaultMinTokenLength.
	MinLength int
	// StopWords drops exact normalized token matches.
	StopWords map[string]struct{}
}

// DefaultTokenizer returns the package default tokenizer.
func DefaultTokenizer() Tokenizer {
	return Tokenizer{MinLength: DefaultMinTokenLength}
}

// NewStopWordSet returns a normalized stop-word set suitable for Tokenizer.
func NewStopWordSet(words ...string) map[string]struct{} {
	stopWords := make(map[string]struct{}, len(words))
	for _, word := range words {
		for _, token := range DefaultTokenizer().Tokenize(word) {
			stopWords[token] = struct{}{}
		}
	}
	return stopWords
}

// Tokenize tokenizes text with the default tokenizer.
func Tokenize(text string) []string {
	return DefaultTokenizer().Tokenize(text)
}

// Tokenize tokenizes text into lowercase letter/digit terms.
func (t Tokenizer) Tokenize(text string) []string {
	minLength := t.MinLength
	if minLength <= 0 {
		minLength = DefaultMinTokenLength
	}

	tokens := make([]string, 0, len(text)/6)
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		token := b.String()
		b.Reset()
		if utf8.RuneCountInString(token) < minLength {
			return
		}
		if _, blocked := t.StopWords[token]; blocked {
			return
		}
		tokens = append(tokens, token)
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()

	return tokens
}
