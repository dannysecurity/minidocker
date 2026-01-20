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
