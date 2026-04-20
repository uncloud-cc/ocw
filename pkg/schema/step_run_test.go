package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestExpose_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(*testing.T, *Expose)
		wantErr bool
	}{
		{
			name: "single int port",
			yaml: "expose: 8080",
			check: func(t *testing.T, e *Expose) {
				if len(e.Ports) != 1 {
					t.Fatalf("expected 1 port, got %d", len(e.Ports))
				}
				if e.Ports[0].ContainerPort != 8080 {
					t.Errorf("expected ContainerPort 8080, got %d", e.Ports[0].ContainerPort)
				}
				if e.Ports[0].HostPort != 8080 {
					t.Errorf("expected HostPort 8080, got %d", e.Ports[0].HostPort)
				}
				if e.Ports[0].Protocol != "http" {
					t.Errorf("expected Protocol 'http', got %q", e.Ports[0].Protocol)
				}
			},
			wantErr: false,
		},
		{
			name: "array of ints",
			yaml: "expose:\n- 8080\n- 9229",
			check: func(t *testing.T, e *Expose) {
				if len(e.Ports) != 2 {
					t.Fatalf("expected 2 ports, got %d", len(e.Ports))
				}
				if e.Ports[0].ContainerPort != 8080 {
					t.Errorf("expected first ContainerPort 8080, got %d", e.Ports[0].ContainerPort)
				}
				if e.Ports[1].ContainerPort != 9229 {
					t.Errorf("expected second ContainerPort 9229, got %d", e.Ports[1].ContainerPort)
				}
				if e.Ports[0].Protocol != "http" || e.Ports[1].Protocol != "http" {
					t.Error("expected Protocol 'http' for both ports")
				}
			},
			wantErr: false,
		},
		{
			name: "array of ExposePort objects",
			yaml: "expose:\n- containerPort: 3000\n  hostPort: 80\n  protocol: http\n- containerPort: 443\n  protocol: https",
			check: func(t *testing.T, e *Expose) {
				if len(e.Ports) != 2 {
					t.Fatalf("expected 2 ports, got %d", len(e.Ports))
				}
				// First port
				if e.Ports[0].ContainerPort != 3000 {
					t.Errorf("expected ContainerPort 3000, got %d", e.Ports[0].ContainerPort)
				}
				if e.Ports[0].HostPort != 80 {
					t.Errorf("expected HostPort 80, got %d", e.Ports[0].HostPort)
				}
				if e.Ports[0].Protocol != "http" {
					t.Errorf("expected Protocol 'http', got %q", e.Ports[0].Protocol)
				}
				// Second port
				if e.Ports[1].ContainerPort != 443 {
					t.Errorf("expected ContainerPort 443, got %d", e.Ports[1].ContainerPort)
				}
				if e.Ports[1].HostPort != 443 {
					t.Errorf("expected HostPort 443 (default), got %d", e.Ports[1].HostPort)
				}
				if e.Ports[1].Protocol != "https" {
					t.Errorf("expected Protocol 'https', got %q", e.Ports[1].Protocol)
				}
			},
			wantErr: false,
		},
		{
			name: "object with defaults",
			yaml: "expose:\n- containerPort: 8080",
			check: func(t *testing.T, e *Expose) {
				if len(e.Ports) != 1 {
					t.Fatalf("expected 1 port, got %d", len(e.Ports))
				}
				if e.Ports[0].HostPort != 8080 {
					t.Errorf("expected HostPort to default to ContainerPort (8080), got %d", e.Ports[0].HostPort)
				}
				if e.Ports[0].Protocol != "http" {
					t.Errorf("expected Protocol to default to 'http', got %q", e.Ports[0].Protocol)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj struct {
				Expose *Expose `yaml:"expose"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && obj.Expose != nil {
				tt.check(t, obj.Expose)
			}
		})
	}
}

func TestExpose_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		expose   *Expose
		expected string
	}{
		{
			name: "single simple port",
			expose: &Expose{
				Ports: []ExposePort{
					{ContainerPort: 8080, HostPort: 8080, Protocol: "http"},
				},
			},
			expected: "expose: 8080\n",
		},
		{
			name: "multiple simple ports",
			expose: &Expose{
				Ports: []ExposePort{
					{ContainerPort: 8080, HostPort: 8080, Protocol: "http"},
					{ContainerPort: 9229, HostPort: 9229, Protocol: "http"},
				},
			},
			expected: "expose:\n- 8080\n- 9229\n",
		},
		{
			name: "port with custom host port",
			expose: &Expose{
				Ports: []ExposePort{
					{ContainerPort: 3000, HostPort: 80, Protocol: "http"},
				},
			},
			expected: "expose:\n- containerPort: 3000\n  hostPort: 80\n  protocol: http\n",
		},
		{
			name: "port with https protocol",
			expose: &Expose{
				Ports: []ExposePort{
					{ContainerPort: 443, HostPort: 443, Protocol: "https"},
				},
			},
			expected: "expose:\n- containerPort: 443\n  hostPort: 443\n  protocol: https\n",
		},
		{
			name:     "nil expose",
			expose:   nil,
			expected: "expose: null\n",
		},
		{
			name: "empty ports",
			expose: &Expose{
				Ports: []ExposePort{},
			},
			expected: "expose: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Expose *Expose `yaml:"expose"`
			}{
				Expose: tt.expose,
			}
			data, err := yaml.Marshal(&obj)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalYAML() = %q; want %q", string(data), tt.expected)
			}
		})
	}
}
