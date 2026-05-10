package embedding

import (
	"math"
	"sort"
)

// Score is a ranked identifier.
type Score struct {
	ID    string
	Score float64
}

// TopK returns a score-descending, ID-ascending copy capped to k entries.
func TopK(scores []Score, k int) []Score {
	if k <= 0 || len(scores) == 0 {
		return nil
	}
	ranked := append([]Score(nil), scores...)
	sort.Slice(ranked, func(i, j int) bool {
		return scoreLess(ranked[i], ranked[j])
	})
	if k > len(ranked) {
		k = len(ranked)
	}
	return ranked[:k]
}

func scoreLess(a, b Score) bool {
	aNaN := math.IsNaN(a.Score)
	bNaN := math.IsNaN(b.Score)
	if aNaN || bNaN {
		if aNaN != bNaN {
			return !aNaN
		}
		return a.ID < b.ID
	}
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	return a.ID < b.ID
}
