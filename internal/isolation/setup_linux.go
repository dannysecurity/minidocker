//go:build linux

package isolation

import (
	"fmt"
	"os"
	"syscall"
)

// PrepareRootfs configures the container view of the filesystem inside an
// already-unshared mount namespace. It sets the UTS hostname, marks mounts
// private, chroots into rootfs when provided, and mounts a fresh procfs.
func PrepareRootfs(rootfs, hostname string) error {
	if hostname != "" {
		if err := syscall.Sethostname([]byte(hostname)); err != nil {
			return fmt.Errorf("set hostname: %w", err)
		}
	}

	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}

	if rootfs != "" {
		if err := syscall.Chroot(rootfs); err != nil {
			return fmt.Errorf("chroot %q: %w", rootfs, err)
		}
		if err := os.Chdir("/"); err != nil {
			return fmt.Errorf("chdir / after chroot: %w", err)
		}
	}

	if err := os.MkdirAll("/proc", 0555); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount proc: %w", err)
	}

	return nil
}
