package embedding

import (
	"errors"
	"math"
)

// ErrDimensionMismatch reports incompatible dense vector lengths.
var ErrDimensionMismatch = errors.New("embedding: dense vector dimensions differ")

// SparseVector maps token terms to numeric weights.
type SparseVector map[string]float64

// DenseVector stores provider-supplied dense embeddings.
type DenseVector []float64

// Counts returns raw token counts as a sparse vector.
func Counts(tokens []string) SparseVector {
	vector := make(SparseVector, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		vector[token]++
	}
	return vector
}

// TermFrequency returns length-normalized token frequencies.
func TermFrequency(tokens []string) SparseVector {
	counts := Counts(tokens)
	if len(tokens) == 0 {
		return counts
	}
	scale := float64(len(tokens))
	for token, count := range counts {
		counts[token] = count / scale
	}
	return counts
}

// ApplyIDF multiplies term-frequency weights by inverse document frequency.
func ApplyIDF(tf SparseVector, idf SparseVector) SparseVector {
	weighted := make(SparseVector, len(tf))
	for token, weight := range tf {
		if idfWeight, ok := idf[token]; ok {
			weighted[token] = weight * idfWeight
			continue
		}
		weighted[token] = weight
	}
	return weighted
}

// Clone returns an independent copy of v.
func (v SparseVector) Clone() SparseVector {
	if v == nil {
		return nil
	}
	clone := make(SparseVector, len(v))
	for key, value := range v {
		clone[key] = value
	}
	return clone
}

// Norm returns the Euclidean norm of v.
func (v SparseVector) Norm() float64 {
	var sum float64
	for _, value := range v {
		sum += value * value
	}
	return math.Sqrt(sum)
}

// Normalize returns a unit-length copy of v. Empty or zero vectors return an
// empty non-nil vector.
func (v SparseVector) Normalize() SparseVector {
	norm := v.Norm()
	if norm == 0 {
		return SparseVector{}
	}
	normalized := make(SparseVector, len(v))
	for key, value := range v {
		normalized[key] = value / norm
	}
	return normalized
}

// Dot returns the sparse dot product of a and b.
func Dot(a, b SparseVector) float64 {
	if len(a) > len(b) {
		a, b = b, a
	}
	var sum float64
	for token, av := range a {
		sum += av * b[token]
	}
	return sum
}

// Cosine returns cosine similarity between sparse vectors.
func Cosine(a, b SparseVector) float64 {
	an := a.Norm()
	bn := b.Norm()
	if an == 0 || bn == 0 {
		return 0
	}
	return Dot(a, b) / (an * bn)
}

// DenseDot returns the dense dot product of a and b.
func DenseDot(a, b DenseVector) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum, nil
}

// DenseCosine returns cosine similarity between dense vectors.
func DenseCosine(a, b DenseVector) (float64, error) {
	dot, err := DenseDot(a, b)
	if err != nil {
		return 0, err
	}
	var an, bn float64
	for _, value := range a {
		an += value * value
	}
	for _, value := range b {
		bn += value * value
	}
	if an == 0 || bn == 0 {
		return 0, nil
	}
	return dot / (math.Sqrt(an) * math.Sqrt(bn)), nil
}
