package image

import (
	"os"
	"path/filepath"
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

func TestInstallFromTarMissingFile(t *testing.T) {
	store := NewStore(t.TempDir())

	err := store.InstallFromTar("tiny:latest", "/nonexistent/tiny-rootfs.tar.gz")
	if err == nil {
		t.Fatal("expected error for missing tarball")
	}
}
