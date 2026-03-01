package isolation

import "syscall"

// Profile selects which Linux namespaces the isolation wrapper creates.
// The zero value isolates UTS, PID, mount, IPC, and network namespaces.
type Profile struct {
	// HostNetwork shares the host network namespace (CLONE_NEWNET is omitted).
	HostNetwork bool
	// HostPID shares the host PID namespace (CLONE_NEWPID is omitted).
	HostPID bool
}

// CloneFlags returns the effective clone(2) flags for this profile.
func (p Profile) CloneFlags() uintptr {
	flags := uintptr(
		syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC,
	)
	if !p.HostPID {
		flags |= syscall.CLONE_NEWPID
	}
	if !p.HostNetwork {
		flags |= syscall.CLONE_NEWNET
	}
	return flags
}
