//go:build linux

package isolation

import "syscall"

const prSetNoNewPrivs = 38

// SetNoNewPrivileges prevents the process from gaining privileges through
// setuid/setgid binaries or file capabilities.
func SetNoNewPrivileges() error {
	_, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
