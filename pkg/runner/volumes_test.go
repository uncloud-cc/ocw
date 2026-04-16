package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestRunner_resolveVolumes(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name      string
		volumes   schema.Volumes
		setupFunc func() string // returns workflowDir
		wantErr   bool
		validate  func(*testing.T, *Runner)
	}{
		{
			name: "absolute path",
			volumes: schema.Volumes{
				"data": schema.Volume{
					Path: testDir,
					Mode: schema.VolumeModeReadOnly,
				},
			},
			setupFunc: func() string {
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, r *Runner) {
				vol, ok := r.resolvedVolumes["data"]
				if !ok {
					t.Fatal("volume 'data' not resolved")
				}
				if vol.HostPath != testDir {
					t.Errorf("expected HostPath %q, got %q", testDir, vol.HostPath)
				}
				if vol.Mode != schema.VolumeModeReadOnly {
					t.Errorf("expected Mode %q, got %q", schema.VolumeModeReadOnly, vol.Mode)
				}
			},
		},
		{
			name: "relative path",
			volumes: schema.Volumes{
				"rel": schema.Volume{
					Path: "testdir",
					Mode: schema.VolumeModeReadWrite,
				},
			},
			setupFunc: func() string {
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, r *Runner) {
				vol, ok := r.resolvedVolumes["rel"]
				if !ok {
					t.Fatal("volume 'rel' not resolved")
				}
				if vol.HostPath != testDir {
					t.Errorf("expected HostPath %q, got %q", testDir, vol.HostPath)
				}
				if vol.Mode != schema.VolumeModeReadWrite {
					t.Errorf("expected Mode %q, got %q", schema.VolumeModeReadWrite, vol.Mode)
				}
			},
		},
		{
			name: "default mode to readonly",
			volumes: schema.Volumes{
				"data": schema.Volume{
					Path: testDir,
				},
			},
			setupFunc: func() string {
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, r *Runner) {
				vol, ok := r.resolvedVolumes["data"]
				if !ok {
					t.Fatal("volume 'data' not resolved")
				}
				if vol.Mode != schema.VolumeModeReadOnly {
					t.Errorf("expected default Mode to be %q, got %q", schema.VolumeModeReadOnly, vol.Mode)
				}
			},
		},
		{
			name: "volume with mountPath",
			volumes: schema.Volumes{
				"data": schema.Volume{
					Path:      testDir,
					MountPath: "/custom/path",
				},
			},
			setupFunc: func() string {
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, r *Runner) {
				vol, ok := r.resolvedVolumes["data"]
				if !ok {
					t.Fatal("volume 'data' not resolved")
				}
				if vol.MountPath != "/custom/path" {
					t.Errorf("expected MountPath %q, got %q", "/custom/path", vol.MountPath)
				}
			},
		},
		{
			name: "nonexistent path",
			volumes: schema.Volumes{
				"missing": schema.Volume{
					Path: filepath.Join(tmpDir, "nonexistent"),
				},
			},
			setupFunc: func() string {
				return tmpDir
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				WorkflowDir: tt.setupFunc(),
			}

			err := r.resolveVolumes(tt.volumes)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, r)
			}
		})
	}
}

func TestRunner_resolveVolumeRef(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		resolvedVolumes map[string]*ResolvedVolume
		ref             schema.VolumeRef
		wantMount       VolumeMount
		wantErr         bool
	}{
		{
			name: "basic volume reference",
			resolvedVolumes: map[string]*ResolvedVolume{
				"data": {
					Name:     "data",
					HostPath: tmpDir,
					Mode:     schema.VolumeModeReadOnly,
				},
			},
			ref: schema.VolumeRef{
				Name: "data",
			},
			wantMount: VolumeMount{
				HostPath:      tmpDir,
				ContainerPath: "/volumes/data",
				ReadOnly:      true,
			},
			wantErr: false,
		},
		{
			name: "volume with custom mount path in volume definition",
			resolvedVolumes: map[string]*ResolvedVolume{
				"data": {
					Name:      "data",
					HostPath:  tmpDir,
					Mode:      schema.VolumeModeReadOnly,
					MountPath: "/custom/vol",
				},
			},
			ref: schema.VolumeRef{
				Name: "data",
			},
			wantMount: VolumeMount{
				HostPath:      tmpDir,
				ContainerPath: "/custom/vol",
				ReadOnly:      true,
			},
			wantErr: false,
		},
		{
			name: "volume ref overrides mount path",
			resolvedVolumes: map[string]*ResolvedVolume{
				"data": {
					Name:      "data",
					HostPath:  tmpDir,
					Mode:      schema.VolumeModeReadOnly,
					MountPath: "/custom/vol",
				},
			},
			ref: schema.VolumeRef{
				Name:      "data",
				MountPath: "/override",
			},
			wantMount: VolumeMount{
				HostPath:      tmpDir,
				ContainerPath: "/override",
				ReadOnly:      true,
			},
			wantErr: false,
		},
		{
			name: "read-write volume can be mounted as read-only",
			resolvedVolumes: map[string]*ResolvedVolume{
				"data": {
					Name:     "data",
					HostPath: tmpDir,
					Mode:     schema.VolumeModeReadWrite,
				},
			},
			ref: schema.VolumeRef{
				Name:     "data",
				ReadOnly: boolPtr(true),
			},
			wantMount: VolumeMount{
				HostPath:      tmpDir,
				ContainerPath: "/volumes/data",
				ReadOnly:      true,
			},
			wantErr: false,
		},
		{
			name: "read-write volume mounted as read-write",
			resolvedVolumes: map[string]*ResolvedVolume{
				"data": {
					Name:     "data",
					HostPath: tmpDir,
					Mode:     schema.VolumeModeReadWrite,
				},
			},
			ref: schema.VolumeRef{
				Name:     "data",
				ReadOnly: boolPtr(false),
			},
			wantMount: VolumeMount{
				HostPath:      tmpDir,
				ContainerPath: "/volumes/data",
				ReadOnly:      false,
			},
			wantErr: false,
		},
		{
			name: "read-only volume cannot be mounted as read-write",
			resolvedVolumes: map[string]*ResolvedVolume{
				"data": {
					Name:     "data",
					HostPath: tmpDir,
					Mode:     schema.VolumeModeReadOnly,
				},
			},
			ref: schema.VolumeRef{
				Name:     "data",
				ReadOnly: boolPtr(false),
			},
			wantErr: true,
		},
		{
			name: "undefined volume reference",
			resolvedVolumes: map[string]*ResolvedVolume{
				"other": {
					Name:     "other",
					HostPath: tmpDir,
					Mode:     schema.VolumeModeReadOnly,
				},
			},
			ref: schema.VolumeRef{
				Name: "missing",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				resolvedVolumes: tt.resolvedVolumes,
			}

			mount, err := r.resolveVolumeRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveVolumeRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if mount.HostPath != tt.wantMount.HostPath {
					t.Errorf("HostPath = %q, want %q", mount.HostPath, tt.wantMount.HostPath)
				}
				if mount.ContainerPath != tt.wantMount.ContainerPath {
					t.Errorf("ContainerPath = %q, want %q", mount.ContainerPath, tt.wantMount.ContainerPath)
				}
				if mount.ReadOnly != tt.wantMount.ReadOnly {
					t.Errorf("ReadOnly = %v, want %v", mount.ReadOnly, tt.wantMount.ReadOnly)
				}
			}
		})
	}
}

func TestRunner_resolveStepVolumes(t *testing.T) {
	tmpDir := t.TempDir()

	r := &Runner{
		resolvedVolumes: map[string]*ResolvedVolume{
			"vol1": {
				Name:     "vol1",
				HostPath: filepath.Join(tmpDir, "vol1"),
				Mode:     schema.VolumeModeReadOnly,
			},
			"vol2": {
				Name:     "vol2",
				HostPath: filepath.Join(tmpDir, "vol2"),
				Mode:     schema.VolumeModeReadWrite,
			},
			"vol3": {
				Name:     "vol3",
				HostPath: filepath.Join(tmpDir, "vol3"),
				Mode:     schema.VolumeModeReadOnly,
			},
		},
	}

	tests := []struct {
		name        string
		stepVolumes schema.VolumeRefs
		jobVolumes  schema.VolumeRefs
		wantCount   int
		wantErr     bool
		validate    func(*testing.T, []VolumeMount)
	}{
		{
			name: "step volumes only",
			stepVolumes: schema.VolumeRefs{
				{Name: "vol1"},
				{Name: "vol2"},
			},
			jobVolumes: nil,
			wantCount:  2,
			wantErr:    false,
		},
		{
			name:        "job volumes only",
			stepVolumes: nil,
			jobVolumes: schema.VolumeRefs{
				{Name: "vol1"},
				{Name: "vol2"},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "step volumes override job volumes (no duplicates)",
			stepVolumes: schema.VolumeRefs{
				{Name: "vol1", MountPath: "/step/vol1"},
			},
			jobVolumes: schema.VolumeRefs{
				{Name: "vol1", MountPath: "/job/vol1"}, // Should be ignored
				{Name: "vol2"},
			},
			wantCount: 2,
			wantErr:   false,
			validate: func(t *testing.T, mounts []VolumeMount) {
				// First mount should be vol1 from step (with step's mount path)
				if mounts[0].ContainerPath != "/step/vol1" {
					t.Errorf("expected first mount path to be /step/vol1, got %q", mounts[0].ContainerPath)
				}
			},
		},
		{
			name:        "empty volumes",
			stepVolumes: schema.VolumeRefs{},
			jobVolumes:  schema.VolumeRefs{},
			wantCount:   0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts, err := r.resolveStepVolumes(tt.stepVolumes, tt.jobVolumes)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveStepVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(mounts) != tt.wantCount {
					t.Errorf("expected %d mounts, got %d", tt.wantCount, len(mounts))
				}
				if tt.validate != nil {
					tt.validate(t, mounts)
				}
			}
		})
	}
}
