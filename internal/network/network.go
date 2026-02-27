package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const DefaultBridge = "minidocker0"

// Manager handles container networking via Linux bridges and veth pairs.
type Manager struct {
	bridge string
}

// NewManager creates a network manager for the given bridge interface.
func NewManager(bridge string) *Manager {
	return &Manager{bridge: bridge}
}

// Setup creates a veth pair and attaches it to the bridge for a container.
// It returns the allocated container IP address.
func (m *Manager) Setup(containerID string, pid int) (string, error) {
	if err := m.ensureBridge(); err != nil {
		return "", fmt.Errorf("ensure bridge: %w", err)
	}

	hostVeth := hostVethName(containerID)
	containerVeth := "eth0"

	if err := run("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", containerVeth); err != nil {
		return "", fmt.Errorf("create veth: %w", err)
	}

	nsPath := filepath.Join("/proc", fmt.Sprintf("%d", pid), "ns", "net")
	if err := run("ip", "link", "set", containerVeth, "netns", nsPath); err != nil {
		_ = run("ip", "link", "del", hostVeth)
		return "", fmt.Errorf("move veth to namespace: %w", err)
	}

	if err := run("ip", "link", "set", hostVeth, "master", m.bridge); err != nil {
		return "", fmt.Errorf("attach to bridge: %w", err)
	}
	if err := run("ip", "link", "set", hostVeth, "up"); err != nil {
		return "", fmt.Errorf("bring up host veth: %w", err)
	}

	containerIP := m.ContainerIP(containerID)
	commands := [][]string{
		{"ip", "link", "set", "lo", "up"},
		{"ip", "link", "set", containerVeth, "up"},
		{"ip", "addr", "add", containerIP + "/24", "dev", containerVeth},
		{"ip", "route", "add", "default", "via", "172.17.0.1"},
	}
	for _, args := range commands {
		if err := runInNetNS(nsPath, args...); err != nil {
			return "", fmt.Errorf("configure container net: %w", err)
		}
	}

	return containerIP, nil
}

// Teardown removes the host-side veth and any port-mapping iptables rules.
func (m *Manager) Teardown(containerID, containerIP string, mappings []PortMapping) {
	_ = m.RemovePortMappings(containerIP, mappings)
	_ = run("ip", "link", "del", hostVethName(containerID))
}

// ContainerIP returns the deterministic bridge IP assigned to a container ID.
func (m *Manager) ContainerIP(containerID string) string {
	return m.allocateIP(containerID)
}

func hostVethName(containerID string) string {
	return fmt.Sprintf("veth%s", containerID[:8])
}

func (m *Manager) ensureBridge() error {
	if err := run("ip", "link", "show", m.bridge); err == nil {
		return nil
	}

	if err := run("ip", "link", "add", m.bridge, "type", "bridge"); err != nil {
		return err
	}
	if err := run("ip", "addr", "add", "172.17.0.1/16", "dev", m.bridge); err != nil {
		return err
	}
	return run("ip", "link", "set", m.bridge, "up")
}

func (m *Manager) allocateIP(containerID string) string {
	var sum byte
	for _, c := range containerID {
		sum += byte(c)
	}
	return fmt.Sprintf("172.17.0.%d", 2+(int(sum)%250))
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runInNetNS(nsPath string, args ...string) error {
	nsenterArgs := append([]string{"--net=" + nsPath}, args...)
	cmd := exec.Command("nsenter", nsenterArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
