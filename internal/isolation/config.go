package isolation

import (
	"fmt"
	"syscall"
)

// DefaultCloneFlags are the Linux namespace flags applied when starting a container.
var DefaultCloneFlags = uintptr(
	syscall.CLONE_NEWUTS |
		syscall.CLONE_NEWPID |
		syscall.CLONE_NEWNS |
		syscall.CLONE_NEWIPC |
		syscall.CLONE_NEWNET,
)

// Config describes how the isolation wrapper should start a container workload.
type Config struct {
	// ID is the container identifier; it becomes the UTS hostname inside the namespace.
	ID string
	// Rootfs is the path to the unpacked image root filesystem. When empty the
	// workload runs on the host filesystem inside the new namespaces.
	Rootfs string
	// Command is the process to exec after namespace and rootfs setup.
	Command []string
	// CloneFlags overrides DefaultCloneFlags when non-zero.
	CloneFlags uintptr
}

// Validate checks that the config has the minimum fields required to start.
func (c Config) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("isolation: container id required")
	}
	if len(c.Command) == 0 {
		return fmt.Errorf("isolation: command required")
	}
	if c.Command[0] == "" {
		return fmt.Errorf("isolation: command binary required")
	}
	return nil
}

// cloneFlags returns the effective clone flags for this config.
func (c Config) cloneFlags() uintptr {
	if c.CloneFlags != 0 {
		return c.CloneFlags
	}
	return DefaultCloneFlags
}
