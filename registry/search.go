package registry

import (
	"sort"
	"strings"
	"unicode"
)

// ToolSearchResult represents a tool match with relevance score.
type ToolSearchResult struct {
	Tool      ToolDefinition
	Score     int
	MatchType string // "name", "tag", "search_term", "category", "runtime_group", "description"
}

const (
	searchWeightName         = 90
	searchWeightTag          = 80
	searchWeightSearchTerm   = 75
	searchWeightCategory     = 60
	searchWeightRuntimeGroup = 60
	searchWeightDescription  = 30
)

var searchStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "do": {}, "does": {}, "did": {},
	"please": {}, "can": {}, "could": {}, "would": {}, "should": {}, "me": {},
	"my": {}, "our": {}, "us": {}, "on": {}, "in": {}, "at": {}, "by": {},
	"for": {}, "from": {}, "into": {}, "of": {}, "to": {}, "with": {},
	"and": {}, "or": {}, "then": {}, "this": {}, "that": {}, "these": {},
	"those": {}, "it": {}, "its": {},
}

type searchField struct {
	matchType string
	weight    int
	tokens    []string
}

type searchTokenMatch struct {
	score     int
	matchType string
	strong    bool
}

type rankedToolSearchResult struct {
	result               ToolSearchResult
	exactWholeName       bool
	exactWholeSearchTerm bool
	allTokensMatched     bool
	matchedCount         int
}

// SearchTools searches for tools matching a natural-language query.
//
// Search is deliberately token based: punctuation (including '_' and '-') is
// a separator, stopwords are discarded, and each query token receives only its
// best field match. This prevents repeated metadata from multiplying a weak
// signal while still allowing a partially specified multi-word request to
// return useful candidates.
func (r *ToolRegistry) SearchTools(query string) []ToolSearchResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	queryTokensAll := searchTokens(query)
	if len(queryTokensAll) == 0 {
		return nil
	}
	queryTokens := filterSearchStopwords(queryTokensAll)
	if len(queryTokens) == 0 {
		return nil
	}
	queryPhrase := strings.Join(queryTokensAll, " ")

	ranked := make([]rankedToolSearchResult, 0, len(r.tools))
	for _, tool := range r.tools {
		fields := toolSearchFields(tool)
		matchedCount := 0
		strongCount := 0
		totalScore := 0
		bestMatch := searchTokenMatch{}

		for _, queryToken := range queryTokens {
			match := bestSearchTokenMatch(queryToken, fields)
			if match.score == 0 {
				continue
			}
			matchedCount++
			totalScore += match.score
			if match.strong {
				strongCount++
			}
			if match.score > bestMatch.score {
				bestMatch = match
			}
		}

		if !passesSearchFloor(len(queryTokens), matchedCount, strongCount) {
			continue
		}

		exactWholeName := strings.Join(searchTokens(tool.Tool.Name), " ") == queryPhrase
		exactWholeSearchTerm := false
		for _, term := range tool.SearchTerms {
			if strings.Join(searchTokens(term), " ") == queryPhrase {
				exactWholeSearchTerm = true
				break
			}
		}

		matchType := bestMatch.matchType
		if exactWholeName {
			matchType = "name"
		} else if exactWholeSearchTerm {
			matchType = "search_term"
		}
		ranked = append(ranked, rankedToolSearchResult{
			result: ToolSearchResult{
				Tool:      tool,
				Score:     totalScore,
				MatchType: matchType,
			},
			exactWholeName:       exactWholeName,
			exactWholeSearchTerm: exactWholeSearchTerm,
			allTokensMatched:     matchedCount == len(queryTokens),
			matchedCount:         matchedCount,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.exactWholeName != right.exactWholeName {
			return left.exactWholeName
		}
		if left.exactWholeSearchTerm != right.exactWholeSearchTerm {
			return left.exactWholeSearchTerm
		}
		if left.allTokensMatched != right.allTokensMatched {
			return left.allTokensMatched
		}
		if left.matchedCount != right.matchedCount {
			return left.matchedCount > right.matchedCount
		}
		if left.result.Score != right.result.Score {
			return left.result.Score > right.result.Score
		}
		return left.result.Tool.Tool.Name < right.result.Tool.Tool.Name
	})

	results := make([]ToolSearchResult, len(ranked))
	for i := range ranked {
		results[i] = ranked[i].result
	}
	return results
}

func toolSearchFields(tool ToolDefinition) []searchField {
	fields := []searchField{{
		matchType: "name",
		weight:    searchWeightName,
		tokens:    searchTokens(tool.Tool.Name),
	}}
	for _, tag := range tool.Tags {
		fields = append(fields, searchField{
			matchType: "tag",
			weight:    searchWeightTag,
			tokens:    searchTokens(tag),
		})
	}
	for _, term := range tool.SearchTerms {
		fields = append(fields, searchField{
			matchType: "search_term",
			weight:    searchWeightSearchTerm,
			tokens:    searchTokens(term),
		})
	}
	fields = append(fields,
		searchField{
			matchType: "category",
			weight:    searchWeightCategory,
			tokens:    searchTokens(tool.Category),
		},
		searchField{
			matchType: "runtime_group",
			weight:    searchWeightRuntimeGroup,
			tokens:    searchTokens(tool.RuntimeGroup),
		},
		searchField{
			matchType: "description",
			weight:    searchWeightDescription,
			tokens:    searchTokens(tool.Tool.Description),
		},
	)
	return fields
}

func bestSearchTokenMatch(queryToken string, fields []searchField) searchTokenMatch {
	best := searchTokenMatch{}
	for _, field := range fields {
		for _, candidateToken := range field.tokens {
			score, strong := scoreSearchToken(queryToken, candidateToken, field.weight)
			if score > best.score {
				best = searchTokenMatch{
					score:     score,
					matchType: field.matchType,
					strong:    strong,
				}
			}
		}
	}
	return best
}

func scoreSearchToken(queryToken, candidateToken string, weight int) (score int, strong bool) {
	if queryToken == candidateToken {
		return weight, true
	}

	queryLen := len([]rune(queryToken))
	candidateLen := len([]rune(candidateToken))
	if queryLen >= 5 && candidateLen >= queryLen && strings.HasPrefix(candidateToken, queryToken) && queryLen*100 >= candidateLen*60 {
		return weight / 2, true
	}

	maxDistance := 0
	switch {
	case queryLen >= 8:
		maxDistance = 2
	case queryLen >= 4:
		maxDistance = 1
	}
	if maxDistance == 0 || absInt(queryLen-candidateLen) > maxDistance {
		return 0, false
	}
	distance := levenshtein(queryToken, candidateToken)
	if distance == 0 || distance > maxDistance {
		return 0, false
	}
	// Fuzzy matches are recovery evidence, never peers of exact or prefix
	// matches. Keep them positive so a single-token typo remains discoverable.
	return weight / 3, false
}

func passesSearchFloor(queryCount, matchedCount, strongCount int) bool {
	if matchedCount == 0 {
		return false
	}
	if queryCount == 1 {
		return true
	}
	if strongCount == 0 {
		return false
	}
	if queryCount == 2 {
		return matchedCount >= 1
	}
	return matchedCount >= 2
}

func filterSearchStopwords(tokens []string) []string {
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, stopword := searchStopwords[token]; stopword {
			continue
		}
		filtered = append(filtered, token)
	}
	return filtered
}

// searchTokens lowercases text and splits it at every non-letter/non-digit
// rune. In particular, underscores and hyphens do not create opaque tokens.
func searchTokens(text string) []string {
	var tokens []string
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		tokens = append(tokens, token.String())
		token.Reset()
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			token.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return tokens
}

// levenshtein computes the edit distance between two Unicode strings.
func levenshtein(a, b string) int {
	aRunes := []rune(a)
	bRunes := []rune(b)
	if len(aRunes) == 0 {
		return len(bRunes)
	}
	if len(bRunes) == 0 {
		return len(aRunes)
	}

	prev := make([]int, len(bRunes)+1)
	curr := make([]int, len(bRunes)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(aRunes); i++ {
		curr[0] = i
		for j := 1; j <= len(bRunes); j++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}
			insert := curr[j-1] + 1
			delete := prev[j] + 1
			substitute := prev[j-1] + cost
			minimum := insert
			if delete < minimum {
				minimum = delete
			}
			if substitute < minimum {
				minimum = substitute
			}
			curr[j] = minimum
		}
		prev, curr = curr, prev
	}
	return prev[len(bRunes)]
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
