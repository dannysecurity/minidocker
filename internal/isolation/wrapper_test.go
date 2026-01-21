package isolation

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid",
			cfg: Config{
				ID:      "abc123",
				Rootfs:  "/tmp/rootfs",
				Command: []string{"/bin/echo", "hi"},
			},
		},
		{
			name: "missing id",
			cfg: Config{
				Command: []string{"/bin/echo"},
			},
			wantErr: true,
		},
		{
			name: "missing command",
			cfg: Config{
				ID: "abc123",
			},
			wantErr: true,
		},
		{
			name: "empty command binary",
			cfg: Config{
				ID:      "abc123",
				Command: []string{""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestDefaultCloneFlags(t *testing.T) {
	want := uintptr(
		syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET,
	)
	if DefaultCloneFlags != want {
		t.Fatalf("DefaultCloneFlags = %#x, want %#x", DefaultCloneFlags, want)
	}
}

func TestWrapperCommand(t *testing.T) {
	initPath := filepath.Join(t.TempDir(), initBinaryName)
	if err := os.WriteFile(initPath, []byte{0x7f, 'E', 'L', 'F'}, 0755); err != nil {
		t.Fatalf("write fake init: %v", err)
	}

	w := &Wrapper{InitPath: initPath}
	cmd, err := w.Command(Config{
		ID:      "deadbeef0001",
		Rootfs:  "/var/lib/minidocker/images/tiny/rootfs",
		Command: []string{"/bin/echo", "hello"},
	})
	if err != nil {
		t.Fatalf("Command: %v", err)
	}

	if cmd.Path != initPath {
		t.Fatalf("cmd.Path = %q, want %q", cmd.Path, initPath)
	}

	wantArgs := []string{
		initPath,
		"--rootfs", "/var/lib/minidocker/images/tiny/rootfs",
		"--hostname", "deadbeef0001",
		"--",
		"/bin/echo", "hello",
	}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, wantArgs)
	}
	for i := range wantArgs {
		if cmd.Args[i] != wantArgs[i] {
			t.Fatalf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], wantArgs[i])
		}
	}

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr")
	}
	if cmd.SysProcAttr.Cloneflags != DefaultCloneFlags {
		t.Fatalf("Cloneflags = %#x, want %#x", cmd.SysProcAttr.Cloneflags, DefaultCloneFlags)
	}

	foundRootfs := false
	foundHostname := false
	for _, env := range cmd.Env {
		if env == "MINIDOCKER_ROOTFS=/var/lib/minidocker/images/tiny/rootfs" {
			foundRootfs = true
		}
		if env == "MINIDOCKER_HOSTNAME=deadbeef0001" {
			foundHostname = true
		}
	}
	if !foundRootfs || !foundHostname {
		t.Fatalf("cmd.Env missing MINIDOCKER_* vars: %v", cmd.Env)
	}
}

func TestWrapperCommandCustomCloneFlags(t *testing.T) {
	initPath := filepath.Join(t.TempDir(), initBinaryName)
	if err := os.WriteFile(initPath, []byte{0x7f, 'E', 'L', 'F'}, 0755); err != nil {
		t.Fatalf("write fake init: %v", err)
	}

	flags := uintptr(syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID)
	cmd, err := (&Wrapper{InitPath: initPath}).Command(Config{
		ID:         "abc",
		Command:    []string{"/bin/true"},
		CloneFlags: flags,
	})
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if cmd.SysProcAttr.Cloneflags != flags {
		t.Fatalf("Cloneflags = %#x, want %#x", cmd.SysProcAttr.Cloneflags, flags)
	}
}

func TestWrapperResolveInitPathMissing(t *testing.T) {
	w := &Wrapper{}
	if _, err := w.resolveInitPath(); err == nil {
		t.Fatal("expected error when init binary is missing")
	}
}
