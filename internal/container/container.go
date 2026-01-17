package container

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dannysecurity/minidocker/internal/log"
	"github.com/dannysecurity/minidocker/internal/network"
)

const DefaultRoot = "/var/lib/minidocker/containers"

// RunSpec describes how to start a container.
type RunSpec struct {
	Image   string
	Rootfs  string
	Command []string
}

// Info holds metadata about a running or stopped container.
type Info struct {
	ID      string `json:"id"`
	Image   string `json:"image"`
	Command string `json:"command"`
	Status  string `json:"status"`
	PID     int    `json:"pid"`
	Created time.Time `json:"created"`
}

// Runtime manages container lifecycle.
type Runtime struct {
	root   string
	logger *log.Logger
}

// NewRuntime creates a container runtime.
func NewRuntime(root string, logger *log.Logger) *Runtime {
	return &Runtime{root: root, logger: logger}
}

// Run starts a new container process inside Linux namespaces.
// It returns the new container ID.
func (r *Runtime) Run(spec RunSpec) (string, error) {
	id := generateID()
	containerDir := filepath.Join(r.root, id)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return "", fmt.Errorf("create container dir: %w", err)
	}

	info := Info{
		ID:      id,
		Image:   spec.Image,
		Command: strings.Join(spec.Command, " "),
		Status:  "running",
		Created: time.Now(),
	}

	cmd := exec.Command(spec.Command[0], spec.Command[1:]...)
	procAttr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET,
	}
	if spec.Rootfs != "" {
		procAttr.Chroot = spec.Rootfs
	}
	cmd.SysProcAttr = procAttr

	var stdout, stderr io.Writer = os.Stdout, os.Stderr
	if r.logger != nil {
		logOut, logErr, err := r.logger.Attach(id)
		if err != nil {
			return "", fmt.Errorf("attach logger: %w", err)
		}
		defer logOut.Close()
		defer logErr.Close()
		stdout, stderr = logOut, logErr
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	// Child process: set up mount namespace and pivot into rootfs.
	cmd.Env = append(os.Environ(),
		"MINIDOCKER_ROOTFS="+spec.Rootfs,
		"MINIDOCKER_HOSTNAME="+id,
	)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}

	info.PID = cmd.Process.Pid

	// Set up networking in the container's network namespace.
	netMgr := network.NewManager(network.DefaultBridge)
	if err := netMgr.Setup(id, info.PID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: network setup failed: %v\n", err)
	}

	if err := r.saveInfo(containerDir, info); err != nil {
		return id, err
	}

	fmt.Printf("Container %s started (pid %d)\n", id, info.PID)

	if err := cmd.Wait(); err != nil {
		info.Status = "exited"
	} else {
		info.Status = "exited"
	}
	_ = r.saveInfo(containerDir, info)

	return id, nil
}

// List returns all known containers.
func (r *Runtime) List() ([]Info, error) {
	entries, err := os.ReadDir(r.root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var containers []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := r.loadInfo(filepath.Join(r.root, entry.Name()))
		if err != nil {
			continue
		}
		containers = append(containers, info)
	}
	return containers, nil
}

// Stop sends SIGTERM to a running container.
func (r *Runtime) Stop(id string) error {
	info, err := r.loadInfo(filepath.Join(r.root, id))
	if err != nil {
		return fmt.Errorf("container %q not found", id)
	}
	if info.PID == 0 {
		return fmt.Errorf("container %q has no pid", id)
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}

	info.Status = "stopped"
	return r.saveInfo(filepath.Join(r.root, id), info)
}

func (r *Runtime) saveInfo(dir string, info Info) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}

func (r *Runtime) loadInfo(dir string) (Info, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return Info{}, err
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, err
	}
	return info, nil
}

func generateID() string {
	return fmt.Sprintf("%012x", time.Now().UnixNano())[:12]
}
