package schema

import (
	"testing"

	"github.com/goccy/go-yaml"
)

func TestVolumeRefsUnmarshalYAML_SingleString(t *testing.T) {
	var refs VolumeRefs
	err := yaml.Unmarshal([]byte("dist"), &refs)
	if err != nil {
		t.Fatalf("Failed to unmarshal single string: %v", err)
	}
	if len(refs) != 1 || refs[0].Name != "dist" {
		t.Errorf("Expected single ref with name 'dist', got %v", refs)
	}
}

func TestVolumeRefsUnmarshalYAML_ArrayOfStrings(t *testing.T) {
	var refs VolumeRefs
	err := yaml.Unmarshal([]byte("[src, dist]"), &refs)
	if err != nil {
		t.Fatalf("Failed to unmarshal array of strings: %v", err)
	}
	if len(refs) != 2 || refs[0].Name != "src" || refs[1].Name != "dist" {
		t.Errorf("Expected two refs with names 'src' and 'dist', got %v", refs)
	}
}

func TestVolumeRefsUnmarshalYAML_ArrayOfObjects(t *testing.T) {
	yamlData := `
- name: src
  mountPath: /code
- name: dist
  readonly: true
`
	var refs VolumeRefs
	err := yaml.Unmarshal([]byte(yamlData), &refs)
	if err != nil {
		t.Fatalf("Failed to unmarshal array of objects: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("Expected 2 refs, got %d", len(refs))
	}
	if refs[0].Name != "src" || refs[0].MountPath != "/code" {
		t.Errorf("Expected first ref with name 'src' and mountPath '/code', got %v", refs[0])
	}
	if refs[1].Name != "dist" || refs[1].ReadOnly == nil || !*refs[1].ReadOnly {
		t.Errorf("Expected second ref with name 'dist' and readonly true, got %v", refs[1])
	}
}

func TestVolumeModeDefaults(t *testing.T) {
	yamlData := `
path: ./src
`
	var vol Volume
	err := yaml.Unmarshal([]byte(yamlData), &vol)
	if err != nil {
		t.Fatalf("Failed to unmarshal volume: %v", err)
	}
	if vol.Mode != "" {
		t.Errorf("Expected empty mode (default), got %q", vol.Mode)
	}
	if vol.Path != "./src" {
		t.Errorf("Expected path './src', got %q", vol.Path)
	}
}

func TestVolumeWithMode(t *testing.T) {
	yamlData := `
path: ./output
mode: readwrite
mountPath: /output
`
	var vol Volume
	err := yaml.Unmarshal([]byte(yamlData), &vol)
	if err != nil {
		t.Fatalf("Failed to unmarshal volume: %v", err)
	}
	if vol.Mode != VolumeModeReadWrite {
		t.Errorf("Expected mode 'readwrite', got %q", vol.Mode)
	}
	if vol.MountPath != "/output" {
		t.Errorf("Expected mountPath '/output', got %q", vol.MountPath)
	}
}

func TestVolumesInOCW(t *testing.T) {
	yamlData := `
schemaVersion: "0.1.0"
name: test-volumes
volumes:
  src:
    path: ./src
  output:
    path: ./output
    mode: readwrite
jobs:
  build:
    volumes:
      - src
    sequence:
      - name: Build
        image: node:20
        volumes:
          - name: output
            readonly: true
        cmd: echo "building"
`
	ocw, err := Parse([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to parse OCW with volumes: %v", err)
	}

	if len(ocw.Volumes) != 2 {
		t.Errorf("Expected 2 volumes, got %d", len(ocw.Volumes))
	}

	srcVol, ok := ocw.Volumes["src"]
	if !ok {
		t.Error("Expected 'src' volume")
	} else {
		if srcVol.Path != "./src" {
			t.Errorf("Expected src path './src', got %q", srcVol.Path)
		}
	}

	outputVol, ok := ocw.Volumes["output"]
	if !ok {
		t.Error("Expected 'output' volume")
	} else {
		if outputVol.Mode != VolumeModeReadWrite {
			t.Errorf("Expected output mode 'readwrite', got %q", outputVol.Mode)
		}
	}

	// Check job-level volumes
	job, ok := ocw.Jobs["build"]
	if !ok {
		t.Fatal("Expected 'build' job")
	}

	if len(job.Volumes) != 1 || job.Volumes[0].Name != "src" {
		t.Errorf("Expected job to have 'src' volume, got %v", job.Volumes)
	}

	// Check step-level volumes
	if len(job.Sequence) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(job.Sequence))
	}

	step := job.Sequence[0]
	if step.RunStep == nil {
		t.Fatal("Expected RunStep")
	}

	if len(step.RunStep.Volumes) != 1 {
		t.Fatalf("Expected 1 step volume, got %d", len(step.RunStep.Volumes))
	}

	if step.RunStep.Volumes[0].Name != "output" {
		t.Errorf("Expected step volume name 'output', got %q", step.RunStep.Volumes[0].Name)
	}

	if step.RunStep.Volumes[0].ReadOnly == nil || !*step.RunStep.Volumes[0].ReadOnly {
		t.Error("Expected step volume to have readonly: true")
	}
}

func TestVolumeRefs_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		refs     VolumeRefs
		expected string
	}{
		{
			name:     "empty refs",
			refs:     VolumeRefs{},
			expected: "refs: null\n",
		},
		{
			name: "single simple ref",
			refs: VolumeRefs{
				{Name: "data"},
			},
			expected: "refs: data\n",
		},
		{
			name: "multiple simple refs",
			refs: VolumeRefs{
				{Name: "src"},
				{Name: "dist"},
			},
			expected: "refs:\n- src\n- dist\n",
		},
		{
			name: "single ref with mountPath",
			refs: VolumeRefs{
				{Name: "data", MountPath: "/custom"},
			},
			expected: "refs:\n- name: data\n  mountPath: /custom\n",
		},
		{
			name: "single ref with readOnly",
			refs: VolumeRefs{
				{Name: "data", ReadOnly: boolPtr(true)},
			},
			expected: "refs:\n- name: data\n  readonly: true\n",
		},
		{
			name: "mixed simple and complex refs",
			refs: VolumeRefs{
				{Name: "src"},
				{Name: "dist", MountPath: "/output"},
			},
			expected: "refs:\n- name: src\n- name: dist\n  mountPath: /output\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := struct {
				Refs VolumeRefs `yaml:"refs"`
			}{
				Refs: tt.refs,
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
