//go:build linux

package isolation

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dannysecurity/minidocker/internal/testutil"
)

func TestContainerInitSetsNoNewPrivileges(t *testing.T) {
	testutil.RequireRoot(t)

	rootfs := t.TempDir()
	buildHelper(t, rootfs, "checknonewprivs", `
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "NoNewPrivs:") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[1] == "1" {
				fmt.Println("nonewprivs-ok")
				return
			}
			fmt.Fprintf(os.Stderr, "unexpected %s\n", line)
			os.Exit(1)
		}
	}
	fmt.Fprintln(os.Stderr, "NoNewPrivs not found")
	os.Exit(1)
}
`)

	runInitCheck(t, rootfs, "harden-test", "/bin/checknonewprivs", "nonewprivs-ok\n")
}

func TestPrepareRootfsMountsDevAndSys(t *testing.T) {
	testutil.RequireRoot(t)

	rootfs := t.TempDir()
	buildHelper(t, rootfs, "checkmounts", `
package main

import (
	"fmt"
	"os"
)

func main() {
	checks := []string{"/dev/null", "/dev/zero", "/sys/class", "/tmp", "/proc/self"}
	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(os.Stderr, "missing %s: %v\n", path, err)
			os.Exit(1)
		}
	}
	fmt.Println("mounts-ok")
}
`)

	runInitCheck(t, rootfs, "mount-test", "/bin/checkmounts", "mounts-ok\n")
}

func TestPrepareRootfsPivotRootHidesHostRoot(t *testing.T) {
	testutil.RequireRoot(t)

	rootfs := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootfs, ".minidocker-root-marker"), []byte("inside"), 0644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	buildHelper(t, rootfs, "checkpivot", `
package main

import (
	"fmt"
	"os"
)

func main() {
	if _, err := os.Stat("/.minidocker-root-marker"); err != nil {
		fmt.Fprintf(os.Stderr, "marker missing after pivot: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("pivot-ok")
}
`)

	runInitCheck(t, rootfs, "pivot-test", "/bin/checkpivot", "pivot-ok\n")
}

func buildHelper(t *testing.T, rootfs, name, source string) {
	t.Helper()

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, name+".go")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatalf("write helper source: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(rootfs, "bin"), 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	out := filepath.Join(rootfs, "bin", name)
	cmd := exec.Command("go", "build", "-o", out, srcPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
}

func runInitCheck(t *testing.T, rootfs, hostname, command, wantOutput string) {
	t.Helper()

	initPath := testutil.BuildContainerInit(t, t.TempDir())
	cmd := exec.Command(initPath,
		"--rootfs", rootfs,
		"--hostname", hostname,
		"--",
		command,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: DefaultCloneFlags}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("container-init: %v\n%s", err, out)
	}
	if string(out) != wantOutput {
		t.Fatalf("output = %q, want %q", out, wantOutput)
	}
}
