package sampling

// RequestOption configures a CreateMessageRequest's parameters.
type RequestOption func(*CreateMessageParams)

// WithSystemPrompt sets the system prompt.
func WithSystemPrompt(s string) RequestOption {
	return func(p *CreateMessageParams) { p.SystemPrompt = s }
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) RequestOption {
	return func(p *CreateMessageParams) { p.Temperature = t }
}

// WithModel sets a model preference hint via the request metadata.
// The actual model selection is up to the client.
func WithModel(model string) RequestOption {
	return func(p *CreateMessageParams) {
		p.Metadata = map[string]any{"preferredModel": model}
	}
}
