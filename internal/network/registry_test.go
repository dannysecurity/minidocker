package network

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeContainerConfig(t *testing.T, root, id string, mappings []PortMapping) {
	t.Helper()

	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cfg := containerConfig{ID: id, PortMappings: mappings}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestLoadPortRegistry(t *testing.T) {
	root := t.TempDir()
	writeContainerConfig(t, root, "abc123", []PortMapping{
		{HostPort: 8080, ContainerPort: 80},
		{HostIP: "127.0.0.1", HostPort: 9090, ContainerPort: 90, Protocol: "udp"},
	})

	reg, err := LoadPortRegistry(root)
	if err != nil {
		t.Fatalf("LoadPortRegistry: %v", err)
	}
	if len(reg.entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(reg.entries))
	}
}

func TestLoadPortRegistryMissingRoot(t *testing.T) {
	reg, err := LoadPortRegistry(t.TempDir() + "/missing")
	if err != nil {
		t.Fatalf("LoadPortRegistry: %v", err)
	}
	if len(reg.entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(reg.entries))
	}
}

func TestPortRegistryCheckConflicts(t *testing.T) {
	root := t.TempDir()
	writeContainerConfig(t, root, "busybox01", []PortMapping{
		{HostPort: 8080, ContainerPort: 80},
	})

	reg, err := LoadPortRegistry(root)
	if err != nil {
		t.Fatalf("LoadPortRegistry: %v", err)
	}

	tests := []struct {
		name    string
		mapping PortMapping
		wantErr bool
	}{
		{
			name:    "same host port tcp",
			mapping: PortMapping{HostPort: 8080, ContainerPort: 8080},
			wantErr: true,
		},
		{
			name:    "different host port",
			mapping: PortMapping{HostPort: 8081, ContainerPort: 80},
		},
		{
			name:    "same port different protocol",
			mapping: PortMapping{HostPort: 8080, ContainerPort: 8080, Protocol: "udp"},
		},
		{
			name:    "localhost bind conflicts with all interfaces",
			mapping: PortMapping{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := reg.CheckConflicts("", []PortMapping{tc.mapping})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected conflict error")
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckConflicts: %v", err)
			}
		})
	}
}

func TestPortRegistryExcludeSelf(t *testing.T) {
	root := t.TempDir()
	writeContainerConfig(t, root, "self0001", []PortMapping{
		{HostPort: 8080, ContainerPort: 80},
	})

	reg, err := LoadPortRegistry(root)
	if err != nil {
		t.Fatalf("LoadPortRegistry: %v", err)
	}

	err = reg.CheckConflicts("self0001", []PortMapping{{HostPort: 8080, ContainerPort: 80}})
	if err != nil {
		t.Fatalf("CheckConflicts for same container: %v", err)
	}
}

func TestBindingsConflict(t *testing.T) {
	tests := []struct {
		name string
		a, b PortMapping
		want bool
	}{
		{
			name: "same all-interfaces tcp",
			a:    PortMapping{HostPort: 80, ContainerPort: 80},
			b:    PortMapping{HostPort: 80, ContainerPort: 8080},
			want: true,
		},
		{
			name: "different ports",
			a:    PortMapping{HostPort: 80, ContainerPort: 80},
			b:    PortMapping{HostPort: 81, ContainerPort: 80},
		},
		{
			name: "same port different protocol",
			a:    PortMapping{HostPort: 53, ContainerPort: 53},
			b:    PortMapping{HostPort: 53, ContainerPort: 53, Protocol: "udp"},
		},
		{
			name: "specific ip vs all interfaces",
			a:    PortMapping{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80},
			b:    PortMapping{HostPort: 8080, ContainerPort: 80},
			want: true,
		},
		{
			name: "different specific ips",
			a:    PortMapping{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80},
			b:    PortMapping{HostIP: "10.0.0.1", HostPort: 8080, ContainerPort: 80},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bindingsConflict(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("bindingsConflict() = %v, want %v", got, tc.want)
			}
		})
	}
}
