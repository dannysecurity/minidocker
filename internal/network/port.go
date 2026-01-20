package network

import (
	"fmt"
	"strconv"
	"strings"
)

// PortMapping describes a host port forwarded to a container port.
type PortMapping struct {
	HostPort      int
	ContainerPort int
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
