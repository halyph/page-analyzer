package cli

import (
	"fmt"
	"strings"

	"github.com/halyph/page-analyzer/internal/domain"
)

// FormatTable formats the analysis result as a human-readable table
func FormatTable(result *domain.AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("                    ANALYSIS RESULTS\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	// URL
	sb.WriteString(fmt.Sprintf("URL:              %s\n", result.URL))
	if result.CacheHit {
		sb.WriteString("                  (cached result)\n")
	}
	sb.WriteString("\n")

	// HTML Version
	sb.WriteString(fmt.Sprintf("HTML Version:     %s\n", result.HTMLVersion))
	sb.WriteString("\n")

	// Title
	sb.WriteString("Page Title:       ")
	if result.Title != "" {
		sb.WriteString(result.Title)
	} else {
		sb.WriteString("(no title)")
	}
	sb.WriteString("\n\n")

	// Headings
	sb.WriteString("Headings:\n")
	headings := []struct {
		level string
		count int
	}{
		{"H1", result.Headings.H1},
		{"H2", result.Headings.H2},
		{"H3", result.Headings.H3},
		{"H4", result.Headings.H4},
		{"H5", result.Headings.H5},
		{"H6", result.Headings.H6},
	}

	for _, h := range headings {
		sb.WriteString(fmt.Sprintf("  %-4s %d\n", h.level+":", h.count))
	}
	sb.WriteString(fmt.Sprintf("  Total: %d\n", result.Headings.Total()))
	sb.WriteString("\n")

	// Links
	sb.WriteString("Links:\n")
	sb.WriteString(fmt.Sprintf("  Internal:       %d\n", result.Links.InternalCount()))
	sb.WriteString(fmt.Sprintf("  External:       %d\n", result.Links.ExternalCount()))
	sb.WriteString(fmt.Sprintf("  Total Found:    %d", result.Links.TotalFound))
	if result.Links.Truncated {
		sb.WriteString(" (truncated)")
	}
	sb.WriteString("\n")

	// Link check results
	if result.Links.CheckResult != nil {
		sb.WriteString(fmt.Sprintf("  Checked:        %d\n", result.Links.CheckResult.Checked))
		sb.WriteString(fmt.Sprintf("  Accessible:     %d\n", result.Links.CheckResult.Accessible))
		sb.WriteString(fmt.Sprintf("  Inaccessible:   %d\n", result.Links.CheckResult.InaccessibleCount()))

		if len(result.Links.CheckResult.Inaccessible) > 0 {
			sb.WriteString("\n  Broken Links:\n")
			for i, linkErr := range result.Links.CheckResult.Inaccessible {
				if i >= 10 {
					remaining := len(result.Links.CheckResult.Inaccessible) - 10
					sb.WriteString(fmt.Sprintf("    ... and %d more\n", remaining))
					break
				}
				sb.WriteString(fmt.Sprintf("    - %s (%s)\n", linkErr.URL, linkErr.Reason))
			}
		}

		sb.WriteString(fmt.Sprintf("  Duration:       %s\n", result.Links.CheckResult.Duration))
	} else if result.Links.CheckStatus == domain.LinkCheckPending {
		sb.WriteString(fmt.Sprintf("  Check Status:   pending (job: %s)\n", result.Links.CheckJobID))
	}
	sb.WriteString("\n")

	// Login Form
	sb.WriteString("Login Form:       ")
	if result.HasLoginForm {
		sb.WriteString("Yes ✓\n")
	} else {
		sb.WriteString("No\n")
	}
	sb.WriteString("\n")

	// Timestamp
	sb.WriteString(fmt.Sprintf("Analyzed At:      %s\n", result.AnalyzedAt.Format("2006-01-02 15:04:05 MST")))

	sb.WriteString("═══════════════════════════════════════════════════════════════\n")

	return sb.String()
}

// FormatCompact formats the analysis result in a compact single-line format
func FormatCompact(result *domain.AnalysisResult) string {
	return fmt.Sprintf(
		"%s | %s | Title: %q | Headings: %d | Links: %d internal, %d external | Login: %v",
		result.URL,
		result.HTMLVersion,
		result.Title,
		result.Headings.Total(),
		result.Links.InternalCount(),
		result.Links.ExternalCount(),
		result.HasLoginForm,
	)
}
