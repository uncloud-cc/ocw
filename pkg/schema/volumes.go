package schema

// VolumeMode defines the access mode for a volume
type VolumeMode string

const (
	VolumeModeReadOnly  VolumeMode = "readonly"
	VolumeModeReadWrite VolumeMode = "readwrite"
)

// Volume defines a named volume that provides access to host filesystem
type Volume struct {
	// Path to the host directory (relative to workflow file or absolute)
	Path string `yaml:"path" json:"path" jsonschema:"required"`

	// Access mode: "readonly" (read-only, default) or "readwrite" (read-write)
	// Default: "readonly" - volumes are read-only unless explicitly set to "readwrite"
	Mode VolumeMode `yaml:"mode,omitempty" json:"mode,omitempty" jsonschema:"enum=readonly,enum=readwrite,default=readonly"`

	// Default mount path inside containers (default: /volumes/<name>)
	// Can be overridden per-step/job in volume references
	MountPath string `yaml:"mountPath,omitempty" json:"mountPath,omitempty"`
}

// Volumes is a map of volume names to their definitions
type Volumes map[string]Volume

// VolumeRef references a named volume when mounting in steps/jobs
type VolumeRef struct {
	// Name of the volume (must match a key in top-level volumes)
	Name string `yaml:"name" json:"name" jsonschema:"required"`

	// Override mount path inside the container for this specific mount
	// If not specified, uses the volume's mountPath or defaults to /volumes/<name>
	MountPath string `yaml:"mountPath,omitempty" json:"mountPath,omitempty"`

	// Force read-only access for this mount, even if volume is defined as readwrite
	// Default: false (uses volume's mode)
	// NOTE: Can only make volumes MORE restrictive (readwrite -> readonly), never less restrictive
	// Setting readonly: false on a volume defined as "readonly" will cause a validation error
	ReadOnly *bool `yaml:"readonly,omitempty" json:"readonly,omitempty"`
}

// VolumeRefs is a list of volume references for a step/job
// Can be specified as:
// - A single string (volume name)
// - An array of strings (volume names)
// - An array of VolumeRef objects (with mount path/readonly overrides)
type VolumeRefs []VolumeRef

// UnmarshalYAML implements custom unmarshaling for VolumeRefs
// to support both string shorthand and full object notation
func (v *VolumeRefs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try single string first
	var single string
	if err := unmarshal(&single); err == nil {
		*v = []VolumeRef{{Name: single}}
		return nil
	}

	// Try array of strings
	var strings []string
	if err := unmarshal(&strings); err == nil {
		refs := make([]VolumeRef, len(strings))
		for i, s := range strings {
			refs[i] = VolumeRef{Name: s}
		}
		*v = refs
		return nil
	}

	// Try array of VolumeRef objects
	var refs []VolumeRef
	if err := unmarshal(&refs); err != nil {
		return err
	}
	*v = refs
	return nil
}

// MarshalYAML implements custom marshaling for VolumeRefs
func (v VolumeRefs) MarshalYAML() (interface{}, error) {
	if len(v) == 0 {
		return nil, nil
	}

	// If all refs are simple (just name, no overrides), return array of strings
	allSimple := true
	for _, ref := range v {
		if ref.MountPath != "" || ref.ReadOnly != nil {
			allSimple = false
			break
		}
	}

	if allSimple {
		if len(v) == 1 {
			return v[0].Name, nil
		}
		names := make([]string, len(v))
		for i, ref := range v {
			names[i] = ref.Name
		}
		return names, nil
	}

	return []VolumeRef(v), nil
}
