package image

import (
	"archive/tar"
	"bytes"
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

func TestExtractTarRejectsPathTraversal(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatalf("mkdir rootfs: %v", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "../escape/evil",
		Typeflag: tar.TypeReg,
		Size:     4,
		Mode:     0644,
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("pwn\n")); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	err := extractTar(&buf, dest)
	if err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid tar path") {
		t.Fatalf("error = %q, want invalid tar path", err)
	}
}

func TestExtractTarAllowsNormalPaths(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatalf("mkdir rootfs: %v", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "bin/echo",
		Typeflag: tar.TypeReg,
		Size:     5,
		Mode:     0755,
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	if err := extractTar(&buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "bin", "echo"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("file contents = %q, want hello", data)
	}
}
