package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dannysecurity/minidocker/internal/testutil"
)

func TestInstallFromTar(t *testing.T) {
	store := NewStore(t.TempDir())
	fixture := testutil.FixturePath(t, "tiny-rootfs.tar.gz")

	if err := store.InstallFromTar("tiny:latest", fixture); err != nil {
		t.Fatalf("InstallFromTar: %v", err)
	}

	rootfs, err := store.RootfsPath("tiny:latest")
	if err != nil {
		t.Fatalf("RootfsPath: %v", err)
	}

	echoPath := filepath.Join(rootfs, "bin", "echo")
	info, err := os.Stat(echoPath)
	if err != nil {
		t.Fatalf("stat %s: %v", echoPath, err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("%s is not executable", echoPath)
	}

	meta, err := os.ReadFile(filepath.Join(filepath.Dir(rootfs), "meta"))
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	if len(meta) == 0 {
		t.Fatal("meta file is empty")
	}
}

func TestRootfsPathMissing(t *testing.T) {
	store := NewStore(t.TempDir())

	_, err := store.RootfsPath("missing:latest")
	if err == nil {
		t.Fatal("expected error for missing image")
	}
}

func TestList(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	fixture := testutil.FixturePath(t, "tiny-rootfs.tar.gz")

	if err := store.InstallFromTar("tiny:latest", fixture); err != nil {
		t.Fatalf("InstallFromTar: %v", err)
	}

	images, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("List() returned %d images, want 1", len(images))
	}
	if images[0].Name != "tiny:latest" {
		t.Fatalf("Name = %q, want tiny:latest", images[0].Name)
	}
	if images[0].Source != "local" {
		t.Fatalf("Source = %q, want local", images[0].Source)
	}
	if !strings.HasPrefix(images[0].Digest, "sha256:") {
		t.Fatalf("Digest = %q, want sha256: prefix", images[0].Digest)
	}
}

func TestListEmpty(t *testing.T) {
	images, err := NewStore(t.TempDir()).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("List() = %v, want empty", images)
	}
}

func TestInstallFromTarMissingFile(t *testing.T) {
	store := NewStore(t.TempDir())

	err := store.InstallFromTar("tiny:latest", "/nonexistent/tiny-rootfs.tar.gz")
	if err == nil {
		t.Fatal("expected error for missing tarball")
	}
}
