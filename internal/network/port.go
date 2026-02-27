package network

import (
	"fmt"
	"strconv"
	"strings"
)

// PortMapping describes a host port forwarded to a container port.
type PortMapping struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

// ValidatePortMappings checks for duplicate host ports in a publish list.
func ValidatePortMappings(mappings []PortMapping) error {
	seen := make(map[int]struct{}, len(mappings))
	for _, mapping := range mappings {
		if _, ok := seen[mapping.HostPort]; ok {
			return fmt.Errorf("duplicate host port %d", mapping.HostPort)
		}
		seen[mapping.HostPort] = struct{}{}
	}
	return nil
}

// FormatPorts renders mappings for ps-style output (e.g. "8080->80/tcp").
func FormatPorts(mappings []PortMapping) string {
	if len(mappings) == 0 {
		return ""
	}
	parts := make([]string, len(mappings))
	for i, mapping := range mappings {
		parts[i] = fmt.Sprintf("%d->%d/tcp", mapping.HostPort, mapping.ContainerPort)
	}
	return strings.Join(parts, ", ")
}

// ParsePortMapping parses a Docker-style publish spec "hostPort:containerPort".
func ParsePortMapping(s string) (PortMapping, error) {
	host, container, ok := strings.Cut(s, ":")
	if !ok || host == "" || container == "" {
		return PortMapping{}, fmt.Errorf("invalid port mapping %q: expected hostPort:containerPort", s)
	}

	hostPort, err := parsePort(host)
	if err != nil {
		return PortMapping{}, fmt.Errorf("invalid host port in %q: %w", s, err)
	}

	containerPort, err := parsePort(container)
	if err != nil {
		return PortMapping{}, fmt.Errorf("invalid container port in %q: %w", s, err)
	}

	return PortMapping{HostPort: hostPort, ContainerPort: containerPort}, nil
}

func parsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("must be a number")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("must be between 1 and 65535")
	}
	return port, nil
}
