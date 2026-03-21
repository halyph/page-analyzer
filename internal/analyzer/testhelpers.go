package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

// loadFixture loads an HTML fixture from testdata directory
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}
