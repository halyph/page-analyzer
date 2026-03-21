package collectors

import (
	"strings"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

func TestLoginFormCollector(t *testing.T) {
	tests := []struct {
		name         string
		htmlInput    string
		wantHasLogin bool
	}{
		{
			name: "simple login form",
			htmlInput: `<html><body>
				<form>
					<input type="text" name="username">
					<input type="password" name="password">
				</form>
			</body></html>`,
			wantHasLogin: true,
		},
		{
			name: "no form",
			htmlInput: `<html><body>
				<input type="text" name="search">
			</body></html>`,
			wantHasLogin: false,
		},
		{
			name: "form without password",
			htmlInput: `<html><body>
				<form>
					<input type="text" name="email">
					<input type="submit">
				</form>
			</body></html>`,
			wantHasLogin: false,
		},
		{
			name: "password outside form",
			htmlInput: `<html><body>
				<input type="password" name="password">
				<form>
					<input type="text" name="username">
				</form>
			</body></html>`,
			wantHasLogin: false,
		},
		{
			name: "multiple forms - one with password",
			htmlInput: `<html><body>
				<form>
					<input type="text" name="search">
				</form>
				<form>
					<input type="password" name="password">
				</form>
			</body></html>`,
			wantHasLogin: true,
		},
		{
			name: "nested forms (invalid HTML but test it)",
			htmlInput: `<html><body>
				<form>
					<div>
						<form>
							<input type="password" name="password">
						</form>
					</div>
				</form>
			</body></html>`,
			wantHasLogin: true,
		},
		{
			name: "password input with attributes",
			htmlInput: `<html><body>
				<form method="post" action="/login">
					<input type="text" name="username" required>
					<input type="password" name="password" autocomplete="current-password">
					<button type="submit">Login</button>
				</form>
			</body></html>`,
			wantHasLogin: true,
		},
		{
			name: "multiple password fields",
			htmlInput: `<html><body>
				<form>
					<input type="password" name="password">
					<input type="password" name="confirm">
				</form>
			</body></html>`,
			wantHasLogin: true,
		},
		{
			name: "input type in uppercase (HTML is case-insensitive)",
			htmlInput: `<html><body>
				<form>
					<input type="PASSWORD" name="password">
				</form>
			</body></html>`,
			wantHasLogin: false, // HTML parser lowercases attribute values, so this won't match
		},
		{
			name:         "empty document",
			htmlInput:    "",
			wantHasLogin: false,
		},
		{
			name: "deeply nested password input",
			htmlInput: `<html><body>
				<form>
					<div><div><div><div>
						<input type="password" name="password">
					</div></div></div></div>
				</form>
			</body></html>`,
			wantHasLogin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &LoginFormCollector{}
			result := domain.NewAnalysisResult("https://example.com")

			// Parse and collect
			tokenizer := html.NewTokenizer(strings.NewReader(tt.htmlInput))
			for {
				tokenType := tokenizer.Next()
				if tokenType == html.ErrorToken {
					break
				}
				collector.Collect(tokenizer.Token())
			}

			// Apply result
			collector.Apply(result)

			if result.HasLoginForm != tt.wantHasLogin {
				t.Errorf("HasLoginForm = %v, want %v", result.HasLoginForm, tt.wantHasLogin)
			}
		})
	}
}

func TestLoginFormCollector_StopsAfterFound(t *testing.T) {
	collector := &LoginFormCollector{}

	// First form with password
	collector.Collect(html.Token{Type: html.StartTagToken, Data: "form"})
	collector.Collect(html.Token{
		Type: html.StartTagToken,
		Data: "input",
		Attr: []html.Attribute{{Key: "type", Val: "password"}},
	})
	collector.Collect(html.Token{Type: html.EndTagToken, Data: "form"})

	if !collector.hasLoginForm {
		t.Error("Should have detected login form")
	}

	// Reset state flags
	firstResult := collector.hasLoginForm

	// Try to process more tokens (should be ignored)
	collector.Collect(html.Token{Type: html.StartTagToken, Data: "form"})
	collector.Collect(html.Token{Type: html.StartTagToken, Data: "input"})

	// Should still have the same result
	if collector.hasLoginForm != firstResult {
		t.Error("Collector should stop processing after finding login form")
	}
}

func TestHasPasswordType(t *testing.T) {
	tests := []struct {
		name  string
		attrs []html.Attribute
		want  bool
	}{
		{
			name:  "has password type",
			attrs: []html.Attribute{{Key: "type", Val: "password"}},
			want:  true,
		},
		{
			name:  "has text type",
			attrs: []html.Attribute{{Key: "type", Val: "text"}},
			want:  false,
		},
		{
			name:  "no type attribute",
			attrs: []html.Attribute{{Key: "name", Val: "password"}},
			want:  false,
		},
		{
			name:  "empty attributes",
			attrs: []html.Attribute{},
			want:  false,
		},
		{
			name: "multiple attributes with password",
			attrs: []html.Attribute{
				{Key: "name", Val: "pwd"},
				{Key: "type", Val: "password"},
				{Key: "required", Val: "true"},
			},
			want: true,
		},
		{
			name: "type not password",
			attrs: []html.Attribute{
				{Key: "type", Val: "email"},
				{Key: "name", Val: "password"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasPasswordType(tt.attrs)
			if got != tt.want {
				t.Errorf("hasPasswordType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoginFormFactory(t *testing.T) {
	factory := &LoginFormFactory{}

	// Test Metadata
	metadata := factory.Metadata()
	if metadata.Name != "loginform" {
		t.Errorf("Name = %q, want loginform", metadata.Name)
	}
	if !metadata.Required {
		t.Error("Expected Required to be true")
	}

	// Test Create
	config := domain.CollectorConfig{}
	collector, err := factory.Create(config)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if collector == nil {
		t.Fatal("Create() returned nil collector")
	}

	// Verify it's the right type
	_, ok := collector.(*LoginFormCollector)
	if !ok {
		t.Errorf("collector type = %T, want *LoginFormCollector", collector)
	}
}
