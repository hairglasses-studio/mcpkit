//go:build !official_sdk

package eval

import (
	"encoding/json"
	"io"
)

// LoadSuiteJSON reads a JSON object with name, cases, and threshold fields,
// and returns a Suite. Scorers are not serializable, so the caller provides
// them separately.
func LoadSuiteJSON(r io.Reader, scorers []Scorer) (Suite, error) {
	var suite Suite
	if err := json.NewDecoder(r).Decode(&suite); err != nil {
		return Suite{}, err
	}
	suite.Scorers = scorers
	return suite, nil
}
