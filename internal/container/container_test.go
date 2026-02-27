package container

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dannysecurity/minidocker/internal/network"
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
		IP:      "172.17.0.42",
		PortMappings: []network.PortMapping{
			{HostPort: 8080, ContainerPort: 80},
		},
	}
	writeContainerConfig(t, root, want.ID, want)

	got, err := rt.Inspect(want.ID)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
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

func TestListFilteredRunningOnly(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	running := Info{
		ID:      "running00001",
		Image:   "tiny:latest",
		Command: "/bin/sh",
		Status:  "running",
	}
	exited := Info{
		ID:      "exited000001",
		Image:   "tiny:latest",
		Command: "/bin/echo hi",
		Status:  "exited",
	}
	writeContainerConfig(t, root, running.ID, running)
	writeContainerConfig(t, root, exited.ID, exited)

	containers, err := rt.ListFiltered(false)
	if err != nil {
		t.Fatalf("ListFiltered: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("ListFiltered(false) returned %d containers, want 1", len(containers))
	}
	if containers[0].ID != running.ID {
		t.Fatalf("ListFiltered()[0].ID = %q, want %q", containers[0].ID, running.ID)
	}
}

func TestResolveID(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	want := Info{
		ID:      "abc123def456",
		Image:   "busybox:latest",
		Command: "/bin/sh",
		Status:  "exited",
	}
	writeContainerConfig(t, root, want.ID, want)

	got, err := rt.ResolveID("abc123")
	if err != nil {
		t.Fatalf("ResolveID: %v", err)
	}
	if got != want.ID {
		t.Fatalf("ResolveID() = %q, want %q", got, want.ID)
	}
}

func TestResolveIDAmbiguous(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	for _, id := range []string{"aaa111111111", "aaa222222222"} {
		writeContainerConfig(t, root, id, Info{ID: id, Status: "exited"})
	}

	_, err := rt.ResolveID("aaa")
	if err == nil {
		t.Fatal("expected ambiguous id error")
	}
}

func TestRemoveExited(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	id := "exited000001"
	writeContainerConfig(t, root, id, Info{
		ID:     id,
		Status: "exited",
	})

	if err := rt.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, id)); !os.IsNotExist(err) {
		t.Fatalf("container dir still exists after Remove")
	}
}

func TestRemoveRunning(t *testing.T) {
	root := t.TempDir()
	rt := NewRuntime(root, nil)

	id := "running00001"
	writeContainerConfig(t, root, id, Info{
		ID:     id,
		Status: "running",
		PID:    os.Getpid(),
	})

	err := rt.Remove(id)
	if err == nil {
		t.Fatal("expected error removing running container")
	}
	if !strings.Contains(err.Error(), "running") {
		t.Fatalf("error = %q, want running message", err)
	}
}

func TestRemoveNotFound(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)

	if err := rt.Remove("missing"); err == nil {
		t.Fatal("expected error for missing container")
	}
}
