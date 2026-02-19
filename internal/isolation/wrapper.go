package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const initBinaryName = "container-init"

// Wrapper builds and configures the process that enters container namespaces.
type Wrapper struct {
	// InitPath overrides the path to the container-init helper binary. When empty
	// the wrapper searches next to the current executable and honors MINIDOCKER_INIT.
	InitPath string
}

// NewWrapper returns a Wrapper with default init resolution.
func NewWrapper() *Wrapper {
	return &Wrapper{}
}

// Command builds an exec.Cmd that starts container-init inside new namespaces.
// The caller is responsible for wiring stdio and calling Start.
func (w *Wrapper) Command(cfg Config) (*exec.Cmd, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	initPath, err := w.resolveInitPath()
	if err != nil {
		return nil, err
	}

	args := []string{
		"--rootfs", cfg.Rootfs,
		"--hostname", cfg.ID,
		"--",
	}
	args = append(args, cfg.Command...)

	cmd := exec.Command(initPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: cfg.cloneFlags(),
	}
	cmd.Env = append(os.Environ(),
		"MINIDOCKER_ROOTFS="+cfg.Rootfs,
		"MINIDOCKER_HOSTNAME="+cfg.ID,
	)
	return cmd, nil
}

func (w *Wrapper) resolveInitPath() (string, error) {
	if w.InitPath != "" {
		return w.InitPath, verifyInitBinary(w.InitPath)
	}
	if p := os.Getenv("MINIDOCKER_INIT"); p != "" {
		return p, verifyInitBinary(p)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve init binary: %w", err)
	}
	candidate := filepath.Join(filepath.Dir(exe), initBinaryName)
	if err := verifyInitBinary(candidate); err == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("resolve init binary: %q not found (build with: go build -o %s ./cmd/container-init)", candidate, initBinaryName)
}

func verifyInitBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("resolve init binary: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("resolve init binary: %q is a directory", path)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("resolve init binary: %q is not executable", path)
	}
	return nil
}
