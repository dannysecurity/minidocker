package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type portEntry struct {
	containerID string
	mapping     PortMapping
}

// PortRegistry tracks published host ports across on-disk containers.
type PortRegistry struct {
	entries []portEntry
}

// LoadPortRegistry scans container metadata and builds a host-port occupancy map.
// Bindings remain reserved until a container is removed, even when stopped.
func LoadPortRegistry(containerRoot string) (*PortRegistry, error) {
	reg := &PortRegistry{}

	entries, err := os.ReadDir(containerRoot)
	if os.IsNotExist(err) {
		return reg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read container root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := filepath.Join(containerRoot, entry.Name(), "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		var cfg containerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if cfg.ID == "" {
			cfg.ID = entry.Name()
		}

		for _, mapping := range cfg.PortMappings {
			reg.entries = append(reg.entries, portEntry{
				containerID: cfg.ID,
				mapping:     mapping,
			})
		}
	}

	return reg, nil
}

// CheckConflicts reports whether any mapping collides with an existing binding.
// excludeID lets a container re-use its own ports during restart scenarios.
func (r *PortRegistry) CheckConflicts(excludeID string, mappings []PortMapping) error {
	for _, candidate := range mappings {
		if err := validateProtocol(candidate.Protocol); err != nil {
			return err
		}
		for _, entry := range r.entries {
			if entry.containerID == excludeID {
				continue
			}
			if bindingsConflict(candidate, entry.mapping) {
				return fmt.Errorf(
					"host port %s already published by container %s",
					formatBinding(candidate),
					entry.containerID,
				)
			}
		}
	}
	return nil
}

// bindingsConflict returns true when two mappings cannot coexist on the host.
func bindingsConflict(a, b PortMapping) bool {
	if a.HostPort != b.HostPort || a.protocol() != b.protocol() {
		return false
	}
	if a.HostIP == "" || b.HostIP == "" {
		return true
	}
	return a.HostIP == b.HostIP
}

// containerConfig is the subset of config.json needed for port registry scans.
type containerConfig struct {
	ID           string        `json:"id"`
	PortMappings []PortMapping `json:"port_mappings"`
}
