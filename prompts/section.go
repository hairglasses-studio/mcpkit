//go:build !official_sdk

package prompts

import "strings"

// Section represents a named fragment of a system prompt with cache control.
type Section struct {
	Name      string
	Content   string
	Cacheable bool // true = stable content suitable for API prompt caching
}

// SectionedPrompt builds a prompt string from ordered sections,
// tracking where the cacheable prefix ends for prompt caching APIs.
type SectionedPrompt struct {
	sections []Section
}

// NewSectionedPrompt creates an empty sectioned prompt builder.
func NewSectionedPrompt() *SectionedPrompt {
	return &SectionedPrompt{}
}

// Add appends a section to the prompt. Sections are rendered in insertion order.
func (sp *SectionedPrompt) Add(s Section) {
	sp.sections = append(sp.sections, s)
}

// Sections returns a copy of the current section list.
func (sp *SectionedPrompt) Sections() []Section {
	out := make([]Section, len(sp.sections))
	copy(out, sp.sections)
	return out
}

// Build assembles the prompt string and returns the byte offset where the
// cacheable prefix ends. Sections are rendered in insertion order. When the
// first non-cacheable section is not the first section, the returned boundary
// includes the separator immediately before that section so prompt[:offset]
// is the complete stable prefix. If all sections are cacheable,
// cacheBoundaryOffset equals len(prompt).
func (sp *SectionedPrompt) Build() (prompt string, cacheBoundaryOffset int) {
	if len(sp.sections) == 0 {
		return "", 0
	}

	var b strings.Builder
	boundaryFound := false

	for i, s := range sp.sections {
		if i > 0 {
			if !s.Cacheable && !boundaryFound {
				cacheBoundaryOffset = b.Len() + 1
				boundaryFound = true
			}
			b.WriteByte('\n')
		} else if !s.Cacheable && !boundaryFound {
			cacheBoundaryOffset = 0
			boundaryFound = true
		}
		b.WriteString(s.Content)
	}

	prompt = b.String()
	if !boundaryFound {
		cacheBoundaryOffset = len(prompt)
	}
	return prompt, cacheBoundaryOffset
}
