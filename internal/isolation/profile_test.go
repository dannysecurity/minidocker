package isolation

import (
	"syscall"
	"testing"
)

func TestProfileCloneFlagsDefault(t *testing.T) {
	if got := (Profile{}).CloneFlags(); got != DefaultCloneFlags {
		t.Fatalf("Profile{}.CloneFlags() = %#x, want DefaultCloneFlags %#x", got, DefaultCloneFlags)
	}
}

func TestProfileCloneFlagsHostNetwork(t *testing.T) {
	want := uintptr(
		syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC,
	)
	got := Profile{HostNetwork: true}.CloneFlags()
	if got != want {
		t.Fatalf("HostNetwork CloneFlags = %#x, want %#x", got, want)
	}
}

func TestProfileCloneFlagsHostPID(t *testing.T) {
	want := uintptr(
		syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET,
	)
	got := Profile{HostPID: true}.CloneFlags()
	if got != want {
		t.Fatalf("HostPID CloneFlags = %#x, want %#x", got, want)
	}
}

func TestProfileCloneFlagsHostNetworkAndPID(t *testing.T) {
	want := uintptr(
		syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC,
	)
	got := Profile{HostNetwork: true, HostPID: true}.CloneFlags()
	if got != want {
		t.Fatalf("HostNetwork+HostPID CloneFlags = %#x, want %#x", got, want)
	}
}
