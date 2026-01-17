//go:build integration

package container

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dannysecurity/minidocker/internal/image"
	"github.com/dannysecurity/minidocker/internal/log"
	"github.com/dannysecurity/minidocker/internal/testutil"
)

func TestIntegration_RunFixtureEcho(t *testing.T) {
	testutil.RequireRoot(t)

	root := t.TempDir()
	imagesRoot := filepath.Join(root, "images")
	containersRoot := filepath.Join(root, "containers")

	store := image.NewStore(imagesRoot)
	fixture := testutil.FixturePath(t, "tiny-rootfs.tar.gz")
	if err := store.InstallFromTar("tiny:latest", fixture); err != nil {
		t.Fatalf("InstallFromTar: %v", err)
	}

	rootfs, err := store.RootfsPath("tiny:latest")
	if err != nil {
		t.Fatalf("RootfsPath: %v", err)
	}

	logger, err := log.NewLogger(containersRoot)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	rt := NewRuntime(containersRoot, logger)
	id, err := rt.Run(RunSpec{
		Image:   "tiny:latest",
		Rootfs:  rootfs,
		Command: []string{"/bin/echo", "hello", "fixture"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if id == "" {
		t.Fatal("Run returned empty container ID")
	}

	logs, err := logger.Read(id)
	if err != nil {
		t.Fatalf("Read logs: %v", err)
	}
	if !strings.Contains(string(logs), "hello fixture") {
		t.Fatalf("logs = %q, want output containing %q", logs, "hello fixture")
	}

	containers, err := rt.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found Info
	for _, c := range containers {
		if c.ID == id {
			found = c
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("container %q not found in List()", id)
	}
	if found.Status != "exited" {
		t.Fatalf("status = %q, want exited", found.Status)
	}
	if found.Image != "tiny:latest" {
		t.Fatalf("image = %q, want tiny:latest", found.Image)
	}
}
