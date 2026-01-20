package container

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeContainerConfig(t *testing.T, root, id string, info Info) {
	t.Helper()

	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir container dir: %v", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestInspect(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	want := Info{
		ID:      "abc123def456",
		Image:   "busybox:latest",
		Command: "/bin/sh",
		Status:  "exited",
		PID:     4242,
		Created: time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC),
	}
	writeContainerConfig(t, root, want.ID, want)

	got, err := rt.Inspect(want.ID)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got != want {
		t.Fatalf("Inspect() = %+v, want %+v", got, want)
	}
}

func TestInspectNotFound(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)

	_, err := rt.Inspect("missing")
	if err == nil {
		t.Fatal("expected error for missing container")
	}
}

func TestListSkipsInvalidEntries(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	valid := Info{
		ID:      "valid0000001",
		Image:   "tiny:latest",
		Command: "/bin/echo hi",
		Status:  "exited",
	}
	writeContainerConfig(t, root, valid.ID, valid)

	if err := os.WriteFile(filepath.Join(root, "not-a-dir"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file entry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "broken"), 0755); err != nil {
		t.Fatalf("mkdir broken dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken", "config.json"), []byte("{"), 0644); err != nil {
		t.Fatalf("write broken config: %v", err)
	}

	containers, err := rt.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("List() returned %d containers, want 1", len(containers))
	}
	if containers[0].ID != valid.ID {
		t.Fatalf("List()[0].ID = %q, want %q", containers[0].ID, valid.ID)
	}
}
