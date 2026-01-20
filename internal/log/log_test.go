package log

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTail(t *testing.T) {
	root := t.TempDir()
	logger, err := NewLogger(root)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	dir := filepath.Join(root, "abc123")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte("hello world"), 0644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}

	got, err := logger.ReadTail("abc123", 5)
	if err != nil {
		t.Fatalf("ReadTail: %v", err)
	}
	if string(got) != "world" {
		t.Fatalf("ReadTail() = %q, want %q", got, "world")
	}
}

func TestReadTailFullWhenZero(t *testing.T) {
	root := t.TempDir()
	logger, err := NewLogger(root)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	dir := filepath.Join(root, "abc123")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "full log output"
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(want), 0644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}

	got, err := logger.ReadTail("abc123", 0)
	if err != nil {
		t.Fatalf("ReadTail: %v", err)
	}
	if string(got) != want {
		t.Fatalf("ReadTail() = %q, want %q", got, want)
	}
}
