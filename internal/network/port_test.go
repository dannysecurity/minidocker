package network

import (
	"testing"
)

func TestParsePortMapping(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PortMapping
		wantErr bool
	}{
		{
			name:  "valid mapping",
			input: "8080:80",
			want:  PortMapping{HostPort: 8080, ContainerPort: 80},
		},
		{
			name:  "high ports",
			input: "65535:65535",
			want:  PortMapping{HostPort: 65535, ContainerPort: 65535},
		},
		{
			name:    "missing container port",
			input:   "8080",
			wantErr: true,
		},
		{
			name:    "empty host port",
			input:   ":80",
			wantErr: true,
		},
		{
			name:    "empty container port",
			input:   "8080:",
			wantErr: true,
		},
		{
			name:    "non-numeric host port",
			input:   "abc:80",
			wantErr: true,
		},
		{
			name:    "port zero",
			input:   "0:80",
			wantErr: true,
		},
		{
			name:    "port out of range",
			input:   "8080:65536",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePortMapping(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePortMapping(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParsePortMapping(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}

func TestValidatePortMappings(t *testing.T) {
	tests := []struct {
		name    string
		input   []PortMapping
		wantErr bool
	}{
		{
			name: "unique host ports",
			input: []PortMapping{
				{HostPort: 8080, ContainerPort: 80},
				{HostPort: 8443, ContainerPort: 443},
			},
		},
		{
			name: "duplicate host port",
			input: []PortMapping{
				{HostPort: 8080, ContainerPort: 80},
				{HostPort: 8080, ContainerPort: 8080},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePortMappings(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePortMappings: %v", err)
			}
		})
	}
}

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name  string
		input []PortMapping
		want  string
	}{
		{name: "empty", input: nil, want: ""},
		{
			name:  "single",
			input: []PortMapping{{HostPort: 8080, ContainerPort: 80}},
			want:  "8080->80/tcp",
		},
		{
			name: "multiple",
			input: []PortMapping{
				{HostPort: 8080, ContainerPort: 80},
				{HostPort: 8443, ContainerPort: 443},
			},
			want: "8080->80/tcp, 8443->443/tcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatPorts(tc.input)
			if got != tc.want {
				t.Fatalf("FormatPorts() = %q, want %q", got, tc.want)
			}
		})
	}
}
