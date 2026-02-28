// container-init is PID 1 inside a minidocker container. It prepares the rootfs,
// execs the user workload, and reaps orphaned children in the PID namespace.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/dannysecurity/minidocker/internal/isolation"
)

func main() {
	rootfs := flag.String("rootfs", "", "path to container root filesystem")
	hostname := flag.String("hostname", "", "UTS namespace hostname")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "container-init: command required after --")
		os.Exit(2)
	}

	if err := isolation.PrepareRootfs(*rootfs, *hostname); err != nil {
		fmt.Fprintf(os.Stderr, "container-init: setup: %v\n", err)
		os.Exit(1)
	}

	if err := isolation.SetNoNewPrivileges(); err != nil {
		fmt.Fprintf(os.Stderr, "container-init: harden: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "container-init: start workload: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for sig := range sigCh {
			_ = cmd.Process.Signal(sig)
		}
	}()

	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, 0, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "container-init: wait: %v\n", err)
			os.Exit(1)
		}
		if pid != cmd.Process.Pid {
			continue
		}
		if status.Signaled() {
			os.Exit(128 + int(status.Signal()))
		}
		os.Exit(status.ExitStatus())
	}
}
