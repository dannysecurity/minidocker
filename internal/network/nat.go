package network

import (
	"fmt"
	"sync"
)

const bridgeSubnet = "172.17.0.0/16"

// natRule describes an iptables invocation for install or removal.
type natRule struct {
	table  string
	chain  string
	args   []string
}

// Manager tracks one-time NAT setup for outbound container traffic.
var natSetup sync.Once
var natSetupErr error

// EnableIPForward turns on IPv4 forwarding so containers can reach external networks.
func EnableIPForward() error {
	return writeSysctl("net/ipv4/ip_forward", "1")
}

// ensureOutboundNAT installs a MASQUERADE rule on the bridge subnet once per process.
func (m *Manager) ensureOutboundNAT() error {
	natSetup.Do(func() {
		if err := EnableIPForward(); err != nil {
			natSetupErr = fmt.Errorf("enable ip_forward: %w", err)
			return
		}
		args := masqueradeArgs(m.bridge)
		if err := run("iptables", args...); err != nil {
			natSetupErr = fmt.Errorf("install masquerade rule: %w", err)
		}
	})
	return natSetupErr
}

// ApplyPortMappings installs DNAT and FORWARD rules for each host→container mapping.
func (m *Manager) ApplyPortMappings(containerIP string, mappings []PortMapping) error {
	if len(mappings) == 0 {
		return nil
	}
	if err := m.ensureOutboundNAT(); err != nil {
		return err
	}

	for _, mapping := range mappings {
		for _, rule := range portMappingRules(containerIP, mapping) {
			args := append([]string{"-t", rule.table, "-A", rule.chain}, rule.args...)
			if err := run("iptables", args...); err != nil {
				return fmt.Errorf("apply port mapping %d->%d: %w", mapping.HostPort, mapping.ContainerPort, err)
			}
		}
	}
	return nil
}

// RemovePortMappings deletes iptables rules previously installed for a container.
func (m *Manager) RemovePortMappings(containerIP string, mappings []PortMapping) error {
	for _, mapping := range mappings {
		for _, rule := range portMappingRules(containerIP, mapping) {
			args := append([]string{"-t", rule.table, "-D", rule.chain}, rule.args...)
			_ = run("iptables", args...)
		}
	}
	return nil
}

func masqueradeArgs(bridge string) []string {
	return []string{
		"-t", "nat",
		"-A", "POSTROUTING",
		"-s", bridgeSubnet,
		"!", "-o", bridge,
		"-j", "MASQUERADE",
	}
}

func portMappingRules(containerIP string, mapping PortMapping) []natRule {
	hostPort := fmt.Sprintf("%d", mapping.HostPort)
	containerPort := fmt.Sprintf("%d", mapping.ContainerPort)
	dest := containerIP + ":" + containerPort

	dnatMatch := []string{
		"-p", "tcp",
		"--dport", hostPort,
		"-j", "DNAT",
		"--to-destination", dest,
	}
	forwardMatch := []string{
		"-p", "tcp",
		"-d", containerIP,
		"--dport", containerPort,
		"-j", "ACCEPT",
	}

	return []natRule{
		{table: "nat", chain: "PREROUTING", args: dnatMatch},
		{table: "nat", chain: "OUTPUT", args: dnatMatch},
		{table: "filter", chain: "FORWARD", args: forwardMatch},
	}
}

func writeSysctl(path, value string) error {
	return run("sysctl", "-w", path+"="+value)
}
