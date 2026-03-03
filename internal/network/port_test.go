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
			name:  "host container mapping",
			input: "8080:80",
			want:  PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
		{
			name:  "shorthand same port",
			input: "80",
			want:  PortMapping{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
		},
		{
			name:  "bind address",
			input: "127.0.0.1:8080:80",
			want: PortMapping{
				HostIP:        "127.0.0.1",
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      "tcp",
			},
		},
		{
			name:  "udp mapping",
			input: "5353:53/udp",
			want: PortMapping{
				HostPort:      5353,
				ContainerPort: 53,
				Protocol:      "udp",
			},
		},
		{
			name:  "bind address udp",
			input: "127.0.0.1:5353:53/udp",
			want: PortMapping{
				HostIP:        "127.0.0.1",
				HostPort:      5353,
				ContainerPort: 53,
				Protocol:      "udp",
			},
		},
		{
			name:  "high ports",
			input: "65535:65535",
			want:  PortMapping{HostPort: 65535, ContainerPort: 65535, Protocol: "tcp"},
		},
		{
			name:    "unsupported protocol",
			input:   "8080:80/sctp",
			wantErr: true,
		},
		{
			name:    "invalid host ip",
			input:   "not-an-ip:8080:80",
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
		{
			name: "same port different protocol",
			input: []PortMapping{
				{HostPort: 53, ContainerPort: 53},
				{HostPort: 53, ContainerPort: 53, Protocol: "udp"},
			},
		},
		{
			name: "duplicate bind address",
			input: []PortMapping{
				{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80},
				{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 90},
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
			name: "bind address and udp",
			input: []PortMapping{
				{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80},
				{HostPort: 5353, ContainerPort: 53, Protocol: "udp"},
			},
			want: "127.0.0.1:8080->80/tcp, 5353->53/udp",
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
