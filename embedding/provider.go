package embedding

import "context"

// SparseProvider embeds text into local or provider-backed sparse vectors.
type SparseProvider interface {
	EmbedSparse(ctx context.Context, text string) (SparseVector, error)
}

// DenseProvider embeds text into provider-backed dense vectors.
type DenseProvider interface {
	EmbedDense(ctx context.Context, text string) (DenseVector, error)
}

// SparseProviderFunc adapts a function to SparseProvider.
type SparseProviderFunc func(context.Context, string) (SparseVector, error)

// EmbedSparse calls f(ctx, text).
func (f SparseProviderFunc) EmbedSparse(ctx context.Context, text string) (SparseVector, error) {
	return f(ctx, text)
}

// DenseProviderFunc adapts a function to DenseProvider.
type DenseProviderFunc func(context.Context, string) (DenseVector, error)

// EmbedDense calls f(ctx, text).
func (f DenseProviderFunc) EmbedDense(ctx context.Context, text string) (DenseVector, error) {
	return f(ctx, text)
}

// LocalProvider embeds text with local tokenization and optional IDF weights.
type LocalProvider struct {
	Tokenizer Tokenizer
	IDF       SparseVector
	Normalize bool
}

// ProviderOption customizes a LocalProvider.
type ProviderOption func(*LocalProvider)

// WithProviderTokenizer sets the tokenizer used by a LocalProvider.
func WithProviderTokenizer(tokenizer Tokenizer) ProviderOption {
	return func(provider *LocalProvider) {
		provider.Tokenizer = tokenizer
	}
}

// WithProviderIDF sets TF-IDF weights used by a LocalProvider.
func WithProviderIDF(idf SparseVector) ProviderOption {
	return func(provider *LocalProvider) {
		provider.IDF = idf.Clone()
	}
}

// WithProviderNormalization toggles unit-length normalization.
func WithProviderNormalization(enabled bool) ProviderOption {
	return func(provider *LocalProvider) {
		provider.Normalize = enabled
	}
}

// NewLocalProvider returns a deterministic sparse provider.
func NewLocalProvider(opts ...ProviderOption) LocalProvider {
	provider := LocalProvider{
		Tokenizer: DefaultTokenizer(),
		Normalize: true,
	}
	for _, opt := range opts {
		opt(&provider)
	}
	return provider
}

// EmbedSparse embeds text with local sparse primitives.
func (provider LocalProvider) EmbedSparse(ctx context.Context, text string) (SparseVector, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tokenizer := provider.Tokenizer
	if tokenizer.MinLength == 0 && tokenizer.StopWords == nil {
		tokenizer = DefaultTokenizer()
	}
	vector := TermFrequency(tokenizer.Tokenize(text))
	if provider.IDF != nil {
		vector = ApplyIDF(vector, provider.IDF)
	}
	if provider.Normalize {
		vector = vector.Normalize()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return vector, nil
}
