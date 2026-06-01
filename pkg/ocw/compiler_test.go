package ocw

import (
	"testing"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestInterpolateStepbase(t *testing.T) {
	tests := []struct {
		name    string
		base    *schema.StepBase
		state   *State
		want    schema.StepBase
		wantErr bool
	}{
		{
			name: "no templates",
			base: &schema.StepBase{
				Name:        "Build",
				ID:          "build",
				Description: "Build the app",
			},
			state: &State{},
			want: schema.StepBase{
				Name:        "Build",
				ID:          "build",
				Description: "Build the app",
			},
		},
		{
			name: "interpolate inputs",
			base: &schema.StepBase{
				Name:        "{{ inputs.name }}",
				ID:          "{{ inputs.id }}",
				Description: "{{ inputs.desc }}",
			},
			state: &State{
				Inputs: map[string]string{
					"name": "Deploy",
					"id":   "deploy",
					"desc": "Deploy to prod",
				},
			},
			want: schema.StepBase{
				Name:        "Deploy",
				ID:          "deploy",
				Description: "Deploy to prod",
			},
		},
		{
			name: "interpolate secrets",
			base: &schema.StepBase{
				Name: "{{ secrets.phase }}",
			},
			state: &State{
				Secrets: map[string]string{
					"phase": "staging",
				},
			},
			want: schema.StepBase{
				Name: "staging",
			},
		},
		{
			name: "interpolate steps output",
			base: &schema.StepBase{
				Name: "{{ steps.build.image }}",
			},
			state: &State{
				Steps: map[string]map[string]string{
					"build": {"image": "my-app:latest"},
				},
			},
			want: schema.StepBase{
				Name: "my-app:latest",
			},
		},
		{
			name: "interpolate env keys and values",
			base: &schema.StepBase{
				Env: map[string]string{
					"{{ inputs.key_prefix }}_PORT": "{{ inputs.port }}",
					"STATIC":                       "value",
				},
			},
			state: &State{
				Inputs: map[string]string{
					"key_prefix": "DB",
					"port":       "5432",
				},
			},
			want: schema.StepBase{
				Env: map[string]string{
					"DB_PORT": "5432",
					"STATIC":  "value",
				},
			},
		},
		{
			name: "interpolate secrets map",
			base: &schema.StepBase{
				Secrets: map[string]string{
					"TOKEN": "{{ secrets.api_key }}",
				},
			},
			state: &State{
				Secrets: map[string]string{
					"api_key": "shh-secret",
				},
			},
			want: schema.StepBase{
				Secrets: map[string]string{
					"TOKEN": "shh-secret",
				},
			},
		},
		{
			name: "interpolate envFile single",
			base: &schema.StepBase{
				EnvFile: &schema.StringOrStringSlice{
					Single: stringPtr("{{ inputs.env_file }}"),
				},
			},
			state: &State{
				Inputs: map[string]string{
					"env_file": "/path/to/.env",
				},
			},
			want: schema.StepBase{
				EnvFile: &schema.StringOrStringSlice{
					Single: stringPtr("/path/to/.env"),
				},
			},
		},
		{
			name: "interpolate envFile multiple",
			base: &schema.StepBase{
				EnvFile: &schema.StringOrStringSlice{
					Multiple: []string{"{{ inputs.a }}", "{{ inputs.b }}"},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"a": "/a/.env",
					"b": "/b/.env",
				},
			},
			want: schema.StepBase{
				EnvFile: &schema.StringOrStringSlice{
					Multiple: []string{"/a/.env", "/b/.env"},
				},
			},
		},
		{
			name: "nil envFile",
			base: &schema.StepBase{
				Name: "test",
			},
			state: &State{},
			want: schema.StepBase{
				Name: "test",
			},
		},
		{
			name: "interpolate needs",
			base: &schema.StepBase{
				Needs: []string{"{{ inputs.service }}"},
			},
			state: &State{
				Inputs: map[string]string{
					"service": "db",
				},
			},
			want: schema.StepBase{
				Needs: []string{"db"},
			},
		},
		{
			name: "config passthrough not interpolated",
			base: &schema.StepBase{
				Config: schema.Config{
					"mytool": {
						"url": "{{ inputs.url }}",
					},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"url": "http://example.com",
				},
			},
			want: schema.StepBase{
				Config: schema.Config{
					"mytool": {
						"url": "{{ inputs.url }}",
					},
				},
			},
		},
		{
			name: "missing input returns error",
			base: &schema.StepBase{
				Name: "{{ inputs.missing }}",
			},
			state:   &State{Inputs: map[string]string{}},
			wantErr: true,
		},
		{
			name: "missing steps output returns error",
			base: &schema.StepBase{
				Name: "{{ steps.build.image }}",
			},
			state:   &State{Steps: map[string]map[string]string{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolateStepbase(tt.base, tt.state)
			if (err != nil) != tt.wantErr {
				t.Fatalf("interpolateStepbase() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.Name != tt.want.Name {
				t.Errorf("Name = %q; want %q", got.Name, tt.want.Name)
			}
			if got.ID != tt.want.ID {
				t.Errorf("ID = %q; want %q", got.ID, tt.want.ID)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q; want %q", got.Description, tt.want.Description)
			}
			if !mapsEqual(got.Env, tt.want.Env) {
				t.Errorf("Env = %v; want %v", got.Env, tt.want.Env)
			}
			if !mapsEqual(got.Secrets, tt.want.Secrets) {
				t.Errorf("Secrets = %v; want %v", got.Secrets, tt.want.Secrets)
			}
			if !stringSlicesEqual(got.Needs, tt.want.Needs) {
				t.Errorf("Needs = %v; want %v", got.Needs, tt.want.Needs)
			}
			if !stringOrStringSliceEqual(got.EnvFile, tt.want.EnvFile) {
				t.Errorf("EnvFile = %v; want %v", got.EnvFile, tt.want.EnvFile)
			}
			if !configEqual(got.Config, tt.want.Config) {
				t.Errorf("Config = %v; want %v", got.Config, tt.want.Config)
			}
		})
	}
}

func TestInterpolateRunStepTemplates(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.RunStep
		state   *State
		check   func(t *testing.T, got *schema.RunStep)
		wantErr bool
	}{
		{
			name: "no templates",
			step: &schema.RunStep{
				StepBase: schema.StepBase{Name: "Run"},
				Image:    "alpine:latest",
				Cmd:      "echo hello",
				Args:     []string{"a", "b"},
			},
			state: &State{},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.Name != "Run" {
					t.Errorf("Name = %q; want %q", got.Name, "Run")
				}
				if got.Image != "alpine:latest" {
					t.Errorf("Image = %q; want %q", got.Image, "alpine:latest")
				}
				if got.Cmd != "echo hello" {
					t.Errorf("Cmd = %q; want %q", got.Cmd, "echo hello")
				}
				if !stringSlicesEqual(got.Args, []string{"a", "b"}) {
					t.Errorf("Args = %v; want %v", got.Args, []string{"a", "b"})
				}
			},
		},
		{
			name: "interpolate image from steps output",
			step: &schema.RunStep{
				Image: "{{ steps.build.image }}",
			},
			state: &State{
				Steps: map[string]map[string]string{
					"build": {"image": "my-app:v1"},
				},
			},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.Image != "my-app:v1" {
					t.Errorf("Image = %q; want %q", got.Image, "my-app:v1")
				}
			},
		},
		{
			name: "interpolate cmd and args",
			step: &schema.RunStep{
				Cmd:  "echo {{ inputs.msg }}",
				Args: []string{"{{ inputs.a }}", "{{ inputs.b }}"},
			},
			state: &State{
				Inputs: map[string]string{
					"msg": "hello",
					"a":   "alpha",
					"b":   "beta",
				},
			},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.Cmd != "echo hello" {
					t.Errorf("Cmd = %q; want %q", got.Cmd, "echo hello")
				}
				if !stringSlicesEqual(got.Args, []string{"alpha", "beta"}) {
					t.Errorf("Args = %v; want %v", got.Args, []string{"alpha", "beta"})
				}
			},
		},
		{
			name: "interpolate entrypoint workdir platform",
			step: &schema.RunStep{
				Entrypoint: "/bin/{{ inputs.shell }}",
				Workdir:    "/app/{{ inputs.version }}",
				Platform:   "linux/{{ inputs.arch }}",
			},
			state: &State{
				Inputs: map[string]string{
					"shell":   "bash",
					"version": "v2",
					"arch":    "amd64",
				},
			},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.Entrypoint != "/bin/bash" {
					t.Errorf("Entrypoint = %q; want %q", got.Entrypoint, "/bin/bash")
				}
				if got.Workdir != "/app/v2" {
					t.Errorf("Workdir = %q; want %q", got.Workdir, "/app/v2")
				}
				if got.Platform != "linux/amd64" {
					t.Errorf("Platform = %q; want %q", got.Platform, "linux/amd64")
				}
			},
		},
		{
			name: "interpolate pull policy",
			step: &schema.RunStep{
				Pull: schema.PullPolicy("{{ inputs.policy }}"),
			},
			state: &State{
				Inputs: map[string]string{
					"policy": "always",
				},
			},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.Pull != schema.PullPolicyAlways {
					t.Errorf("Pull = %q; want %q", got.Pull, schema.PullPolicyAlways)
				}
			},
		},
		{
			name: "healthCheck interpolation",
			step: &schema.RunStep{
				HealthCheck: &schema.HealthCheck{
					Cmd:      "curl {{ inputs.url }}",
					Interval: "10s",
					Retries:  3,
				},
			},
			state: &State{
				Inputs: map[string]string{
					"url": "http://localhost:8080",
				},
			},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.HealthCheck == nil {
					t.Fatalf("HealthCheck is nil")
				}
				if got.HealthCheck.Cmd != "curl http://localhost:8080" {
					t.Errorf("HealthCheck.Cmd = %q; want %q", got.HealthCheck.Cmd, "curl http://localhost:8080")
				}
				if got.HealthCheck.Interval != "10s" {
					t.Errorf("HealthCheck.Interval = %q; want %q", got.HealthCheck.Interval, "10s")
				}
				if got.HealthCheck.Retries != 3 {
					t.Errorf("HealthCheck.Retries = %d; want %d", got.HealthCheck.Retries, 3)
				}
			},
		},
		{
			name: "nil healthCheck",
			step: &schema.RunStep{
				Image:       "alpine",
				HealthCheck: nil,
			},
			state: &State{},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.HealthCheck != nil {
					t.Errorf("HealthCheck = %v; want nil", got.HealthCheck)
				}
			},
		},
		{
			name: "passthrough non-interpolated fields",
			step: &schema.RunStep{
				Image:      "alpine",
				Background: true,
				Quiet:      true,
				TTY:        true,
				Memory:     "512m",
			},
			state: &State{},
			check: func(t *testing.T, got *schema.RunStep) {
				if got.Background != true {
					t.Errorf("Background = %v; want true", got.Background)
				}
				if got.Quiet != true {
					t.Errorf("Quiet = %v; want true", got.Quiet)
				}
				if got.TTY != true {
					t.Errorf("TTY = %v; want true", got.TTY)
				}
				if got.Memory != "512m" {
					t.Errorf("Memory = %q; want %q", got.Memory, "512m")
				}
			},
		},
		{
			name: "missing input returns error",
			step: &schema.RunStep{
				Image: "{{ inputs.missing }}",
			},
			state:   &State{Inputs: map[string]string{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolateRunStepTemplates(tt.step, tt.state)
			if (err != nil) != tt.wantErr {
				t.Fatalf("interpolateRunStepTemplates() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			tt.check(t, got)
		})
	}
}

func TestInterpolateBuildStepTemplates(t *testing.T) {
	tests := []struct {
		name    string
		step    *schema.BuildStep
		state   *State
		check   func(t *testing.T, got *schema.BuildStep)
		wantErr bool
	}{
		{
			name: "no templates",
			step: &schema.BuildStep{
				StepBase: schema.StepBase{Name: "Build"},
				Build: schema.BuildConfig{
					Image:      "my-app",
					Context:    ".",
					Dockerfile: "Dockerfile",
					Target:     "prod",
				},
			},
			state: &State{},
			check: func(t *testing.T, got *schema.BuildStep) {
				if got.Name != "Build" {
					t.Errorf("Name = %q; want %q", got.Name, "Build")
				}
				if got.Build.Image != "my-app" {
					t.Errorf("Image = %q; want %q", got.Build.Image, "my-app")
				}
				if got.Build.Context != "." {
					t.Errorf("Context = %q; want %q", got.Build.Context, ".")
				}
				if got.Build.Dockerfile != "Dockerfile" {
					t.Errorf("Dockerfile = %q; want %q", got.Build.Dockerfile, "Dockerfile")
				}
				if got.Build.Target != "prod" {
					t.Errorf("Target = %q; want %q", got.Build.Target, "prod")
				}
			},
		},
		{
			name: "interpolate core fields",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Image:      "{{ inputs.image }}",
					Context:    "{{ inputs.context }}",
					Dockerfile: "{{ inputs.dockerfile }}",
					Target:     "{{ inputs.target }}",
				},
			},
			state: &State{
				Inputs: map[string]string{
					"image":      "my-app:v2",
					"context":    "/workspace",
					"dockerfile": "Containerfile",
					"target":     "release",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if got.Build.Image != "my-app:v2" {
					t.Errorf("Image = %q; want %q", got.Build.Image, "my-app:v2")
				}
				if got.Build.Context != "/workspace" {
					t.Errorf("Context = %q; want %q", got.Build.Context, "/workspace")
				}
				if got.Build.Dockerfile != "Containerfile" {
					t.Errorf("Dockerfile = %q; want %q", got.Build.Dockerfile, "Containerfile")
				}
				if got.Build.Target != "release" {
					t.Errorf("Target = %q; want %q", got.Build.Target, "release")
				}
			},
		},
		{
			name: "interpolate buildArgs keys and values",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					BuildArgs: map[string]string{
						"{{ inputs.key_prefix }}_VER": "{{ inputs.version }}",
						"STATIC":                    "value",
					},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"key_prefix": "APP",
					"version":    "1.2.3",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !mapsEqual(got.Build.BuildArgs, map[string]string{
					"APP_VER": "1.2.3",
					"STATIC":  "value",
				}) {
					t.Errorf("BuildArgs = %v; want %v", got.Build.BuildArgs, map[string]string{"APP_VER": "1.2.3", "STATIC": "value"})
				}
			},
		},
		{
			name: "interpolate platform single",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Platform: &schema.StringOrStringSlice{
						Single: stringPtr("linux/{{ inputs.arch }}"),
					},
				},
			},
			state: &State{
				Inputs: map[string]string{"arch": "amd64"},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if got.Build.Platform == nil || got.Build.Platform.Single == nil || *got.Build.Platform.Single != "linux/amd64" {
					t.Errorf("Platform.Single = %v; want %q", got.Build.Platform, "linux/amd64")
				}
			},
		},
		{
			name: "interpolate platform multiple",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Platform: &schema.StringOrStringSlice{
						Multiple: []string{"linux/{{ inputs.a }}", "linux/{{ inputs.b }}"},
					},
				},
			},
			state: &State{
				Inputs: map[string]string{"a": "amd64", "b": "arm64"},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !stringSlicesEqual(got.Build.Platform.Multiple, []string{"linux/amd64", "linux/arm64"}) {
					t.Errorf("Platform.Multiple = %v; want %v", got.Build.Platform.Multiple, []string{"linux/amd64", "linux/arm64"})
				}
			},
		},
		{
			name: "interpolate cacheFrom and cacheTo",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					CacheFrom: []string{"{{ inputs.cache_from }}"},
					CacheTo:   []string{"{{ inputs.cache_to }}"},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"cache_from": "type=local,src=/cache",
					"cache_to":   "type=local,dest=/cache",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !stringSlicesEqual(got.Build.CacheFrom, []string{"type=local,src=/cache"}) {
					t.Errorf("CacheFrom = %v; want %v", got.Build.CacheFrom, []string{"type=local,src=/cache"})
				}
				if !stringSlicesEqual(got.Build.CacheTo, []string{"type=local,dest=/cache"}) {
					t.Errorf("CacheTo = %v; want %v", got.Build.CacheTo, []string{"type=local,dest=/cache"})
				}
			},
		},
		{
			name: "interpolate tags",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Tags: []string{"{{ inputs.tag1 }}", "{{ inputs.tag2 }}"},
				},
			},
			state: &State{
				Inputs: map[string]string{"tag1": "v1", "tag2": "latest"},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !stringSlicesEqual(got.Build.Tags, []string{"v1", "latest"}) {
					t.Errorf("Tags = %v; want %v", got.Build.Tags, []string{"v1", "latest"})
				}
			},
		},
		{
			name: "interpolate buildSecrets map",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					BuildSecrets: &schema.BuildSecrets{
						Map: map[string]string{
							"{{ inputs.secret_key }}": "{{ inputs.secret_value }}",
						},
					},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"secret_key":   "npm_token",
					"secret_value": "abc123",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !mapsEqual(got.Build.BuildSecrets.Map, map[string]string{"npm_token": "abc123"}) {
					t.Errorf("BuildSecrets.Map = %v; want %v", got.Build.BuildSecrets.Map, map[string]string{"npm_token": "abc123"})
				}
			},
		},
		{
			name: "interpolate buildSecrets array",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					BuildSecrets: &schema.BuildSecrets{
						Array: []schema.BuildSecretConfig{
							{ID: "{{ inputs.id }}", Src: "{{ inputs.src }}", Env: "{{ inputs.env }}"},
						},
					},
				},
			},
			state: &State{
				Inputs: map[string]string{"id": "token", "src": "/secrets/token", "env": "TOKEN"},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if len(got.Build.BuildSecrets.Array) != 1 {
					t.Fatalf("BuildSecrets.Array len = %d; want 1", len(got.Build.BuildSecrets.Array))
				}
				sc := got.Build.BuildSecrets.Array[0]
				if sc.ID != "token" {
					t.Errorf("BuildSecrets.Array[0].ID = %q; want %q", sc.ID, "token")
				}
				if sc.Src != "/secrets/token" {
					t.Errorf("BuildSecrets.Array[0].Src = %q; want %q", sc.Src, "/secrets/token")
				}
				if sc.Env != "TOKEN" {
					t.Errorf("BuildSecrets.Array[0].Env = %q; want %q", sc.Env, "TOKEN")
				}
			},
		},
		{
			name: "interpolate labels",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Labels: map[string]string{
						"{{ inputs.label_key }}": "{{ inputs.label_value }}",
					},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"label_key":   "version",
					"label_value": "1.0.0",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !mapsEqual(got.Build.Labels, map[string]string{"version": "1.0.0"}) {
					t.Errorf("Labels = %v; want %v", got.Build.Labels, map[string]string{"version": "1.0.0"})
				}
			},
		},
		{
			name: "interpolate annotation map",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Annotation: &schema.StringMapOrSlice{
						Map: map[string]string{
							"{{ inputs.ann_key }}": "{{ inputs.ann_value }}",
						},
					},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"ann_key":   "org.opencontainers.image.source",
					"ann_value": "https://github.com/example/repo",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if got.Build.Annotation == nil || got.Build.Annotation.Map == nil {
					t.Fatalf("Annotation.Map is nil")
				}
				if !mapsEqual(got.Build.Annotation.Map, map[string]string{
					"org.opencontainers.image.source": "https://github.com/example/repo",
				}) {
					t.Errorf("Annotation.Map = %v; want %v", got.Build.Annotation.Map, map[string]string{"org.opencontainers.image.source": "https://github.com/example/repo"})
				}
			},
		},
		{
			name: "interpolate annotation slice",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Annotation: &schema.StringMapOrSlice{
						Slice: []string{"{{ inputs.ann1 }}", "{{ inputs.ann2 }}"},
					},
				},
			},
			state: &State{
				Inputs: map[string]string{"ann1": "a=1", "ann2": "b=2"},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !stringSlicesEqual(got.Build.Annotation.Slice, []string{"a=1", "b=2"}) {
					t.Errorf("Annotation.Slice = %v; want %v", got.Build.Annotation.Slice, []string{"a=1", "b=2"})
				}
			},
		},
		{
			name: "interpolate buildContext",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					BuildContext: map[string]string{
						"{{ inputs.ctx_key }}": "{{ inputs.ctx_value }}",
					},
				},
			},
			state: &State{
				Inputs: map[string]string{
					"ctx_key":   "repo",
					"ctx_value": "./subproject",
				},
			},
			check: func(t *testing.T, got *schema.BuildStep) {
				if !mapsEqual(got.Build.BuildContext, map[string]string{"repo": "./subproject"}) {
					t.Errorf("BuildContext = %v; want %v", got.Build.BuildContext, map[string]string{"repo": "./subproject"})
				}
			},
		},
		{
			name: "nil platform buildSecrets annotation",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Image:        "alpine",
					Platform:     nil,
					BuildSecrets: nil,
					Annotation:   nil,
				},
			},
			state: &State{},
			check: func(t *testing.T, got *schema.BuildStep) {
				if got.Build.Platform != nil {
					t.Errorf("Platform = %v; want nil", got.Build.Platform)
				}
				if got.Build.BuildSecrets != nil {
					t.Errorf("BuildSecrets = %v; want nil", got.Build.BuildSecrets)
				}
				if got.Build.Annotation != nil {
					t.Errorf("Annotation = %v; want nil", got.Build.Annotation)
				}
			},
		},
		{
			name: "passthrough non-interpolated fields",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Image:         "alpine",
					NoCache:       true,
					Push:          true,
					Load:          true,
					Pull:          true,
					Quiet:         true,
					ShmSize:       "128m",
					Progress:      schema.BuildProgressQuiet,
					MetadataFile:  "meta.json",
					IIDFile:       "iid.txt",
				},
			},
			state: &State{},
			check: func(t *testing.T, got *schema.BuildStep) {
				if got.Build.NoCache != true {
					t.Errorf("NoCache = %v; want true", got.Build.NoCache)
				}
				if got.Build.Push != true {
					t.Errorf("Push = %v; want true", got.Build.Push)
				}
				if got.Build.Load != true {
					t.Errorf("Load = %v; want true", got.Build.Load)
				}
				if got.Build.Pull != true {
					t.Errorf("Pull = %v; want true", got.Build.Pull)
				}
				if got.Build.Quiet != true {
					t.Errorf("Quiet = %v; want true", got.Build.Quiet)
				}
				if got.Build.ShmSize != "128m" {
					t.Errorf("ShmSize = %q; want %q", got.Build.ShmSize, "128m")
				}
				if got.Build.Progress != schema.BuildProgressQuiet {
					t.Errorf("Progress = %q; want %q", got.Build.Progress, schema.BuildProgressQuiet)
				}
				if got.Build.MetadataFile != "meta.json" {
					t.Errorf("MetadataFile = %q; want %q", got.Build.MetadataFile, "meta.json")
				}
				if got.Build.IIDFile != "iid.txt" {
					t.Errorf("IIDFile = %q; want %q", got.Build.IIDFile, "iid.txt")
				}
			},
		},
		{
			name: "missing input returns error",
			step: &schema.BuildStep{
				Build: schema.BuildConfig{
					Image: "{{ inputs.missing }}",
				},
			},
			state:   &State{Inputs: map[string]string{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolateBuildStepTemplates(tt.step, tt.state)
			if (err != nil) != tt.wantErr {
				t.Fatalf("interpolateBuildStepTemplates() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			tt.check(t, got)
		})
	}
}

// helpers

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringOrStringSliceEqual(a, b *schema.StringOrStringSlice) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Single != nil && b.Single != nil {
		return *a.Single == *b.Single
	}
	if a.Single != nil || b.Single != nil {
		return false
	}
	return stringSlicesEqual(a.Multiple, b.Multiple)
}

func configEqual(a, b schema.Config) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if len(va) != len(vb) {
			return false
		}
		for kk, vva := range va {
			if vb[kk] != vva {
				return false
			}
		}
	}
	return true
}
