//go:build linux

package isolation

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// PrepareRootfs configures the container view of the filesystem inside an
// already-unshared mount namespace. It sets the UTS hostname, marks mounts
// private, pivots into rootfs when provided, and mounts standard pseudo
// filesystems expected by typical workloads.
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
		if err := pivotRoot(rootfs); err != nil {
			return err
		}
	}

	if err := mountEssentialFilesystems(); err != nil {
		return err
	}

	return nil
}

func pivotRoot(rootfs string) error {
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs: %w", err)
	}

	putOld := filepath.Join(rootfs, ".pivot_root")
	if err := os.MkdirAll(putOld, 0700); err != nil {
		return fmt.Errorf("mkdir pivot_old: %w", err)
	}

	if err := syscall.PivotRoot(rootfs, putOld); err != nil {
		return fmt.Errorf("pivot_root %q: %w", rootfs, err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / after pivot: %w", err)
	}

	putOldPath := "/.pivot_root"
	if err := syscall.Unmount(putOldPath, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root: %w", err)
	}
	if err := os.Remove(putOldPath); err != nil {
		return fmt.Errorf("remove pivot_old: %w", err)
	}
	return nil
}

func mountEssentialFilesystems() error {
	if err := os.MkdirAll("/proc", 0555); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount proc: %w", err)
	}

	if err := os.MkdirAll("/dev", 0755); err != nil {
		return fmt.Errorf("mkdir /dev: %w", err)
	}
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755,size=65536k"); err != nil {
		return fmt.Errorf("mount tmpfs on /dev: %w", err)
	}

	devices := []struct {
		path  string
		mode  uint32
		major int
		minor int
	}{
		{"/dev/null", 0666, 1, 3},
		{"/dev/zero", 0666, 1, 5},
		{"/dev/random", 0666, 1, 8},
		{"/dev/urandom", 0666, 1, 9},
		{"/dev/console", 0600, 5, 1},
	}
	for _, dev := range devices {
		if err := mkCharDevice(dev.path, dev.mode, dev.major, dev.minor); err != nil {
			return err
		}
	}

	if err := os.MkdirAll("/dev/pts", 0755); err != nil {
		return fmt.Errorf("mkdir /dev/pts: %w", err)
	}
	if err := syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620,gid=5"); err != nil {
		return fmt.Errorf("mount devpts: %w", err)
	}
	if err := os.Symlink("pts/ptmx", "/dev/ptmx"); err != nil && !os.IsExist(err) {
		return fmt.Errorf("symlink /dev/ptmx: %w", err)
	}

	if err := os.MkdirAll("/sys", 0555); err != nil {
		return fmt.Errorf("mkdir /sys: %w", err)
	}
	if err := syscall.Mount("sysfs", "/sys", "sysfs", syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount sysfs: %w", err)
	}

	if err := os.MkdirAll("/tmp", 01777); err != nil {
		return fmt.Errorf("mkdir /tmp: %w", err)
	}
	if err := syscall.Mount("tmpfs", "/tmp", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777,size=65536k"); err != nil {
		return fmt.Errorf("mount tmpfs on /tmp: %w", err)
	}

	return nil
}

func mkCharDevice(path string, mode uint32, major, minor int) error {
	if err := syscall.Mknod(path, mode|syscall.S_IFCHR, devNum(major, minor)); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("mknod %q: %w", path, err)
	}
	return nil
}

func devNum(major, minor int) int {
	return (major << 8) | minor
}
