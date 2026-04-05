package registry

import (
	"sort"
	"strings"
)

// ToolSearchResult represents a tool match with relevance score.
type ToolSearchResult struct {
	Tool      ToolDefinition
	Score     int
	MatchType string // "name", "tag", "search_term", "category", "runtime_group", "description"
}

// SearchTools searches for tools matching a query string.
// Supports multi-word queries (all words must match), TF-IDF-style weighting
// for rarer terms, and fuzzy matching within edit distance 2 for typo tolerance.
func (r *ToolRegistry) SearchTools(query string) []ToolSearchResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	queryWords := strings.Fields(query)
	idf := r.buildIDF()

	var results []ToolSearchResult

	for _, tool := range r.tools {
		toolNameLower := strings.ToLower(tool.Tool.Name)
		descLower := strings.ToLower(tool.Tool.Description)
		categoryLower := strings.ToLower(tool.Category)
		runtimeGroupLower := strings.ToLower(tool.RuntimeGroup)

		tagsLower := make([]string, len(tool.Tags))
		for i, t := range tool.Tags {
			tagsLower[i] = strings.ToLower(t)
		}
		searchTermsLower := make([]string, len(tool.SearchTerms))
		for i, term := range tool.SearchTerms {
			searchTermsLower[i] = strings.ToLower(term)
		}

		allWordsMatched := true
		totalScore := 0.0
		bestMatchType := ""

		for _, word := range queryWords {
			wordMatched := false
			wordScore := 0.0

			weight := 1.0
			if idf != nil {
				if w, ok := idf[word]; ok {
					weight = w
				} else {
					weight = 3.0
				}
			}

			// Name match (highest priority)
			if strings.Contains(toolNameLower, word) {
				wordScore += 25 * weight
				wordMatched = true
				if bestMatchType == "" {
					bestMatchType = "name"
				}
			} else if fuzzy := fuzzyMatchSegments(word, toolNameLower); fuzzy > 0 {
				wordScore += float64(10*fuzzy) * weight
				wordMatched = true
				if bestMatchType == "" {
					bestMatchType = "name"
				}
			}

			// Tag match
			for _, tagLower := range tagsLower {
				if tagLower == word {
					wordScore += 40 * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" {
						bestMatchType = "tag"
					}
				} else if strings.Contains(tagLower, word) {
					wordScore += 20 * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" {
						bestMatchType = "tag"
					}
				} else if fuzzy := fuzzyMatchSegments(word, tagLower); fuzzy > 0 {
					wordScore += float64(8*fuzzy) * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" {
						bestMatchType = "tag"
					}
				}
			}

			// Explicit search terms / synonyms
			for _, termLower := range searchTermsLower {
				if termLower == word {
					wordScore += 30 * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" || bestMatchType == "category" {
						bestMatchType = "search_term"
					}
				} else if strings.Contains(termLower, word) {
					wordScore += 16 * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" || bestMatchType == "category" {
						bestMatchType = "search_term"
					}
				} else if fuzzy := fuzzyMatchSegments(word, termLower); fuzzy > 0 {
					wordScore += float64(7*fuzzy) * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" || bestMatchType == "category" {
						bestMatchType = "search_term"
					}
				}
			}

			// Category match
			if strings.Contains(categoryLower, word) {
				wordScore += 15 * weight
				wordMatched = true
				if bestMatchType == "" || bestMatchType == "description" {
					bestMatchType = "category"
				}
			} else if fuzzy := fuzzyMatchSegments(word, categoryLower); fuzzy > 0 {
				wordScore += float64(6*fuzzy) * weight
				wordMatched = true
				if bestMatchType == "" || bestMatchType == "description" {
					bestMatchType = "category"
				}
			}

			// Runtime group match
			if runtimeGroupLower != "" {
				if strings.Contains(runtimeGroupLower, word) {
					wordScore += 18 * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" {
						bestMatchType = "runtime_group"
					}
				} else if fuzzy := fuzzyMatchSegments(word, runtimeGroupLower); fuzzy > 0 {
					wordScore += float64(7*fuzzy) * weight
					wordMatched = true
					if bestMatchType == "" || bestMatchType == "description" {
						bestMatchType = "runtime_group"
					}
				}
			}

			// Description match (lowest priority)
			if len(word) > 2 {
				if strings.Contains(descLower, word) {
					wordScore += 5 * weight
					wordMatched = true
					if bestMatchType == "" {
						bestMatchType = "description"
					}
				} else {
					for _, dw := range strings.Fields(descLower) {
						if len(dw) < 3 {
							continue
						}
						dist := levenshtein(word, dw)
						maxDist := 1
						if len(word) >= 5 || len(dw) >= 5 {
							maxDist = 2
						}
						if dist > 0 && dist <= maxDist {
							wordScore += 3 * weight
							wordMatched = true
							if bestMatchType == "" {
								bestMatchType = "description"
							}
							break
						}
					}
				}
			}

			if !wordMatched {
				allWordsMatched = false
				break
			}
			totalScore += wordScore
		}

		if !allWordsMatched || totalScore <= 0 {
			continue
		}

		if strings.Contains(toolNameLower, query) {
			totalScore += 100
			bestMatchType = "name"
		}

		results = append(results, ToolSearchResult{
			Tool:      tool,
			Score:     int(totalScore),
			MatchType: bestMatchType,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Tool.Tool.Name < results[j].Tool.Tool.Name
	})

	return results
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			min := ins
			if del < min {
				min = del
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// fuzzyMatchSegments checks whether word fuzzy-matches any segment of target
// (split on _, -, space). Returns the fuzzy bonus (0 = no match).
func fuzzyMatchSegments(word, target string) int {
	if len(word) < 3 {
		return 0
	}
	segments := strings.FieldsFunc(target, func(r rune) bool {
		return r == '_' || r == ' ' || r == '-'
	})
	for _, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		dist := levenshtein(word, seg)
		maxDist := 1
		if len(word) >= 5 || len(seg) >= 5 {
			maxDist = 2
		}
		if dist > 0 && dist <= maxDist {
			return maxDist - dist + 1
		}
	}
	return 0
}

// buildIDF builds a simple inverse-document-frequency map across all tool text.
func (r *ToolRegistry) buildIDF() map[string]float64 {
	total := len(r.tools)
	if total == 0 {
		return nil
	}

	docCount := make(map[string]int)
	for _, tool := range r.tools {
		seen := make(map[string]bool)
		text := strings.ToLower(tool.Tool.Name + " " + tool.Tool.Description + " " +
			tool.Category + " " + strings.Join(tool.Tags, " "))
		for _, w := range strings.FieldsFunc(text, func(r rune) bool {
			return r == '_' || r == ' ' || r == '-' || r == ',' || r == '.' || r == '(' || r == ')'
		}) {
			if len(w) > 1 && !seen[w] {
				seen[w] = true
				docCount[w]++
			}
		}
	}

	idf := make(map[string]float64, len(docCount))
	for w, count := range docCount {
		ratio := float64(total) / float64(count)
		if ratio > 10 {
			idf[w] = 3.0
		} else if ratio > 5 {
			idf[w] = 2.0
		} else if ratio > 2 {
			idf[w] = 1.5
		} else {
			idf[w] = 1.0
		}
	}
	return idf
}
