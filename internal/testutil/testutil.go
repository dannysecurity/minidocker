package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// RequireRoot skips the test when not running as root.
func RequireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root privileges")
	}
}

// ModuleRoot returns the absolute path to the module root (directory containing go.mod).
func ModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// FixturePath returns the absolute path to a file under testdata/fixtures.
func FixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(ModuleRoot(t), "testdata", "fixtures", name)
}
