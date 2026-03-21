package cli

import (
	"encoding/json"

	"github.com/halyph/page-analyzer/internal/domain"
)

// FormatJSON formats the analysis result as pretty-printed JSON
func FormatJSON(result *domain.AnalysisResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
