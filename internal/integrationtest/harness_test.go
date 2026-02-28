package integrationtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dannysecurity/minidocker/internal/image"
	"github.com/dannysecurity/minidocker/internal/testutil"
)

func TestFixtureTarballContainsHelpers(t *testing.T) {
	store := image.NewStore(t.TempDir())
	if err := store.InstallFromTar(ImageRef, testutil.FixturePath(t, "tiny-rootfs.tar.gz")); err != nil {
		t.Fatalf("InstallFromTar: %v", err)
	}

	rootfs, err := store.RootfsPath(ImageRef)
	if err != nil {
		t.Fatalf("RootfsPath: %v", err)
	}

	for _, bin := range []string{"echo", "readhostname", "sleep", "tcpecho", "writestderr"} {
		path := filepath.Join(rootfs, "bin", bin)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode()&0111 == 0 {
			t.Fatalf("%s is not executable", path)
		}
	}
}
