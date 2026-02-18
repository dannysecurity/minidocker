// Package integrationtest provides helpers for root-required end-to-end tests
// against the tiny fixture container image.
package integrationtest

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dannysecurity/minidocker/internal/container"
	"github.com/dannysecurity/minidocker/internal/image"
	"github.com/dannysecurity/minidocker/internal/log"
	"github.com/dannysecurity/minidocker/internal/testutil"
)

const (
	// ImageRef is the image tag used by integration tests.
	ImageRef = "tiny:latest"
)

// Env wires an isolated minidocker stack: temp image store, logs, and runtime.
type Env struct {
	Root           string
	ImagesRoot     string
	ContainersRoot string
	Store          *image.Store
	Logger         *log.Logger
	Runtime        *container.Runtime
	Rootfs         string
}

// NewEnv installs the tiny fixture image and returns a ready runtime.
// Callers must invoke testutil.RequireRoot before using this in integration tests.
func NewEnv(t *testing.T) *Env {
	t.Helper()

	root := t.TempDir()
	imagesRoot := filepath.Join(root, "images")
	containersRoot := filepath.Join(root, "containers")

	store := image.NewStore(imagesRoot)
	fixture := testutil.FixturePath(t, "tiny-rootfs.tar.gz")
	if err := store.InstallFromTar(ImageRef, fixture); err != nil {
		t.Fatalf("InstallFromTar: %v", err)
	}

	rootfs, err := store.RootfsPath(ImageRef)
	if err != nil {
		t.Fatalf("RootfsPath: %v", err)
	}

	logger, err := log.NewLogger(containersRoot)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	rt := container.NewRuntime(containersRoot, logger)
	rt.SetIsolationInit(testutil.BuildContainerInit(t, t.TempDir()))

	return &Env{
		Root:           root,
		ImagesRoot:     imagesRoot,
		ContainersRoot: containersRoot,
		Store:          store,
		Logger:         logger,
		Runtime:        rt,
		Rootfs:         rootfs,
	}
}

// WaitForStatus polls Inspect until the container reaches the desired status.
func WaitForStatus(t *testing.T, rt *container.Runtime, id, want string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		info, err := rt.Inspect(id)
		if err == nil {
			last = info.Status
			if last == want {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("container %q status = %q, want %q (timed out after %s)", id, last, want, timeout)
}

// EnsureContainerDead stops a lingering container process so temp dirs can be removed.
func EnsureContainerDead(t *testing.T, rt *container.Runtime, id string) {
	t.Helper()

	info, err := rt.Inspect(id)
	if err != nil || info.PID == 0 {
		return
	}
	if syscall.Kill(info.PID, 0) != nil {
		return
	}

	_ = rt.Stop(id)
	_ = syscall.Kill(info.PID, syscall.SIGKILL)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(info.PID, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
