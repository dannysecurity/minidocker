package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// PortMapping describes a host port forwarded to a container port.
type PortMapping struct {
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"`
}

func (m PortMapping) protocol() string {
	if m.Protocol == "" {
		return "tcp"
	}
	return m.Protocol
}

// bindingKey uniquely identifies a host-side publish binding.
func (m PortMapping) bindingKey() string {
	return fmt.Sprintf("%s:%d/%s", m.HostIP, m.HostPort, m.protocol())
}

// ValidatePortMappings checks for duplicate bindings in a single publish list.
func ValidatePortMappings(mappings []PortMapping) error {
	seen := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		if err := validateProtocol(mapping.Protocol); err != nil {
			return err
		}
		key := mapping.bindingKey()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate port binding %s", formatBinding(mapping))
		}
		seen[key] = struct{}{}
	}
	return nil
}

// FormatPorts renders mappings for ps-style output (e.g. "127.0.0.1:8080->80/tcp").
func FormatPorts(mappings []PortMapping) string {
	if len(mappings) == 0 {
		return ""
	}
	parts := make([]string, len(mappings))
	for i, mapping := range mappings {
		parts[i] = formatBinding(mapping)
	}
	return strings.Join(parts, ", ")
}

func formatBinding(mapping PortMapping) string {
	host := fmt.Sprintf("%d", mapping.HostPort)
	if mapping.HostIP != "" {
		host = mapping.HostIP + ":" + host
	}
	return fmt.Sprintf("%s->%d/%s", host, mapping.ContainerPort, mapping.protocol())
}

// ParsePortMapping parses Docker-style publish specs:
//   - "80" (same host and container port)
//   - "8080:80"
//   - "127.0.0.1:8080:80"
//   - "8080:80/udp"
func ParsePortMapping(s string) (PortMapping, error) {
	proto := "tcp"
	if spec, suffix, ok := strings.Cut(s, "/"); ok {
		s = spec
		switch suffix {
		case "tcp", "udp":
			proto = suffix
		default:
			return PortMapping{}, fmt.Errorf("unsupported protocol %q in %q", suffix, s)
		}
	}

	parts := strings.Split(s, ":")
	switch len(parts) {
	case 1:
		port, err := parsePort(parts[0])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid port mapping %q: %w", s, err)
		}
		return PortMapping{HostPort: port, ContainerPort: port, Protocol: proto}, nil
	case 2:
		hostPort, err := parsePort(parts[0])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid host port in %q: %w", s, err)
		}
		containerPort, err := parsePort(parts[1])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid container port in %q: %w", s, err)
		}
		return PortMapping{HostPort: hostPort, ContainerPort: containerPort, Protocol: proto}, nil
	case 3:
		hostIP := parts[0]
		if hostIP == "" || net.ParseIP(hostIP) == nil {
			return PortMapping{}, fmt.Errorf("invalid host IP %q in %q", parts[0], s)
		}
		hostPort, err := parsePort(parts[1])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid host port in %q: %w", s, err)
		}
		containerPort, err := parsePort(parts[2])
		if err != nil {
			return PortMapping{}, fmt.Errorf("invalid container port in %q: %w", s, err)
		}
		return PortMapping{
			HostIP:        hostIP,
			HostPort:      hostPort,
			ContainerPort: containerPort,
			Protocol:      proto,
		}, nil
	default:
		return PortMapping{}, fmt.Errorf("invalid port mapping %q", s)
	}
}

func validateProtocol(proto string) error {
	switch proto {
	case "", "tcp", "udp":
		return nil
	default:
		return fmt.Errorf("unsupported protocol %q", proto)
	}
}

func parsePort(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("must be a number")
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("must be a number")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("must be between 1 and 65535")
	}
	return port, nil
}
