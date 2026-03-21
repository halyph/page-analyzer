# Test Fixtures

HTML fixtures for analyzer tests.

## Files

- `simple.html` - Basic HTML5 page with title, headings, and links
- `login_form.html` - Page with login form containing password input
- `complex.html` - XHTML page with all heading levels and multiple links
- `malformed.html` - Malformed HTML for testing parser forgiveness
- `redirect_final.html` - Final destination page after redirect
- `walker_simple.html` - Minimal page for walker tests
- `walker_complete.html` - Complete page with all collectors
- `walker_malformed.html` - Malformed HTML for walker tests
- `fetcher_simple.html` - Minimal HTML for fetcher tests

## Usage

```go
func TestExample(t *testing.T) {
    html := loadFixture(t, "simple.html")
    // use html in test
}
```
