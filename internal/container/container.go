package container

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dannysecurity/minidocker/internal/log"
	"github.com/dannysecurity/minidocker/internal/network"
)

const DefaultRoot = "/var/lib/minidocker/containers"

// RunSpec describes how to start a container.
type RunSpec struct {
	Image        string
	Rootfs       string
	Command      []string
	Detach       bool
	PortMappings []network.PortMapping
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
// When Detach is false the call blocks until the container exits; when true it
// returns immediately and updates status in the background.
func (r *Runtime) Run(spec RunSpec) (string, error) {
	id, cmd, closers, err := r.startContainer(spec)
	if err != nil {
		return "", err
	}

	containerDir := filepath.Join(r.root, id)
	info := Info{
		ID:      id,
		Image:   spec.Image,
		Command: strings.Join(spec.Command, " "),
		Status:  "running",
		PID:     cmd.Process.Pid,
		Created: time.Now(),
	}

	netMgr := network.NewManager(network.DefaultBridge)
	if err := netMgr.Setup(id, info.PID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: network setup failed: %v\n", err)
	}

	if err := r.saveInfo(containerDir, info); err != nil {
		closeWriters(closers)
		return id, err
	}

	if spec.Detach {
		fmt.Printf("%s\n", id)
		go r.waitAndFinalize(containerDir, cmd, closers)
		return id, nil
	}

	fmt.Printf("Container %s started (pid %d)\n", id, info.PID)
	_ = cmd.Wait()
	closeWriters(closers)
	info.Status = "exited"
	_ = r.saveInfo(containerDir, info)
	return id, nil
}

func (r *Runtime) startContainer(spec RunSpec) (string, *exec.Cmd, []io.Closer, error) {
	id := generateID()
	containerDir := filepath.Join(r.root, id)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return "", nil, nil, fmt.Errorf("create container dir: %w", err)
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
	var closers []io.Closer
	if r.logger != nil {
		logOut, logErr, err := r.logger.Attach(id)
		if err != nil {
			return "", nil, nil, fmt.Errorf("attach logger: %w", err)
		}
		closers = []io.Closer{logOut, logErr}
		if spec.Detach {
			stdout, stderr = logOut, logErr
		} else {
			stdout = io.MultiWriter(os.Stdout, logOut)
			stderr = io.MultiWriter(os.Stderr, logErr)
		}
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"MINIDOCKER_ROOTFS="+spec.Rootfs,
		"MINIDOCKER_HOSTNAME="+id,
	)

	if err := cmd.Start(); err != nil {
		closeWriters(closers)
		return "", nil, nil, fmt.Errorf("start container: %w", err)
	}

	return id, cmd, closers, nil
}

func (r *Runtime) waitAndFinalize(containerDir string, cmd *exec.Cmd, closers []io.Closer) {
	_ = cmd.Wait()
	closeWriters(closers)
	info, err := r.loadInfo(containerDir)
	if err != nil {
		return
	}
	info.Status = "exited"
	_ = r.saveInfo(containerDir, info)
}

func closeWriters(writers []io.Closer) {
	for _, w := range writers {
		_ = w.Close()
	}
}

// Inspect returns metadata for a single container.
func (r *Runtime) Inspect(id string) (Info, error) {
	info, err := r.loadInfo(filepath.Join(r.root, id))
	if err != nil {
		return Info{}, fmt.Errorf("container %q not found", id)
	}
	return info, nil
}

// List returns all known containers.
func (r *Runtime) List() ([]Info, error) {
	return r.ListFiltered(true)
}

// ListFiltered returns containers. When all is false only running containers are included.
func (r *Runtime) ListFiltered(all bool) ([]Info, error) {
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
		if !all && info.Status != "running" {
			continue
		}
		containers = append(containers, info)
	}
	return containers, nil
}

// ResolveID finds a container ID that matches the given prefix.
func (r *Runtime) ResolveID(prefix string) (string, error) {
	if prefix == "" {
		return "", fmt.Errorf("container id required")
	}

	containers, err := r.List()
	if err != nil {
		return "", err
	}

	var matches []string
	for _, c := range containers {
		if c.ID == prefix || strings.HasPrefix(c.ID, prefix) {
			matches = append(matches, c.ID)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("container %q not found", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous container id %q (matches %d containers)", prefix, len(matches))
	}
}

// Exec runs a command inside a running container's namespaces.
func (r *Runtime) Exec(id string, command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("exec requires a command")
	}

	resolved, err := r.ResolveID(id)
	if err != nil {
		return err
	}

	info, err := r.loadInfo(filepath.Join(r.root, resolved))
	if err != nil {
		return fmt.Errorf("container %q not found", id)
	}
	if info.Status != "running" {
		return fmt.Errorf("container %q is not running (status: %s)", resolved, info.Status)
	}
	if info.PID == 0 {
		return fmt.Errorf("container %q has no pid", resolved)
	}

	if err := syscall.Kill(info.PID, 0); err != nil {
		return fmt.Errorf("container %q process is not running: %w", resolved, err)
	}

	args := append([]string{
		"--target", strconv.Itoa(info.PID),
		"--mount", "--uts", "--ipc", "--net", "--pid",
		"--",
	}, command...)
	cmd := exec.Command("nsenter", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec in container: %w", err)
	}
	return nil
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
