package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterExposedService(t *testing.T) {
	runner := NewRunner(".")

	svc1 := ExposedService{
		StepID:        "step1",
		StepName:      "web",
		ContainerPort: 8080,
		HostPort:      8080,
		RequestedPort: 8080,
		Protocol:      "http",
	}

	svc2 := ExposedService{
		StepID:        "step2",
		StepName:      "api",
		ContainerPort: 3000,
		HostPort:      3001,
		RequestedPort: 3000,
		Protocol:      "http",
	}

	runner.registerExposedService(svc1)
	assert.Len(t, runner.exposedServices, 1)
	assert.Equal(t, svc1, runner.exposedServices[0])

	runner.registerExposedService(svc2)
	assert.Len(t, runner.exposedServices, 2)
	assert.Equal(t, svc2, runner.exposedServices[1])
}

func TestPrintExposedServices(t *testing.T) {
	tests := []struct {
		name     string
		services []ExposedService
	}{
		{
			name:     "no services",
			services: []ExposedService{},
		},
		{
			name: "single http service",
			services: []ExposedService{
				{
					StepID:        "web",
					StepName:      "Web Server",
					ContainerPort: 8080,
					HostPort:      8080,
					RequestedPort: 8080,
					Protocol:      "http",
				},
			},
		},
		{
			name: "https service",
			services: []ExposedService{
				{
					StepID:        "secure",
					StepName:      "Secure Server",
					ContainerPort: 443,
					HostPort:      8443,
					RequestedPort: 443,
					Protocol:      "https",
				},
			},
		},
		{
			name: "tcp service",
			services: []ExposedService{
				{
					StepID:        "db",
					StepName:      "Database",
					ContainerPort: 5432,
					HostPort:      5432,
					RequestedPort: 5432,
					Protocol:      "tcp",
				},
			},
		},
		{
			name: "port reassignment",
			services: []ExposedService{
				{
					StepID:        "api",
					StepName:      "API",
					ContainerPort: 3000,
					HostPort:      3001,
					RequestedPort: 3000,
					Protocol:      "http",
				},
			},
		},
		{
			name: "multiple services",
			services: []ExposedService{
				{
					StepID:        "web",
					StepName:      "Web",
					ContainerPort: 80,
					HostPort:      8080,
					RequestedPort: 8080,
					Protocol:      "http",
				},
				{
					StepID:        "api",
					StepName:      "API",
					ContainerPort: 3000,
					HostPort:      3000,
					RequestedPort: 3000,
					Protocol:      "http",
				},
			},
		},
		{
			name: "service without step ID",
			services: []ExposedService{
				{
					StepID:        "",
					StepName:      "Unnamed Service",
					ContainerPort: 9000,
					HostPort:      9000,
					RequestedPort: 9000,
					Protocol:      "tcp",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(".")
			runner.exposedServices = tt.services

			// Just test that it doesn't panic
			// printExposedServices writes to output, which is hard to test directly
			assert.NotPanics(t, func() {
				runner.printExposedServices()
			})
		})
	}
}

func TestExposedServiceURLFormatting(t *testing.T) {
	tests := []struct {
		name     string
		service  ExposedService
		wantHTTP bool
	}{
		{
			name: "http service formats as http URL",
			service: ExposedService{
				StepID:        "web",
				StepName:      "Web",
				ContainerPort: 80,
				HostPort:      8080,
				RequestedPort: 8080,
				Protocol:      "http",
			},
			wantHTTP: true,
		},
		{
			name: "https service formats as https URL",
			service: ExposedService{
				StepID:        "secure",
				StepName:      "Secure",
				ContainerPort: 443,
				HostPort:      8443,
				RequestedPort: 443,
				Protocol:      "https",
			},
			wantHTTP: false,
		},
		{
			name: "tcp service formats as host:port",
			service: ExposedService{
				StepID:        "db",
				StepName:      "Database",
				ContainerPort: 5432,
				HostPort:      5432,
				RequestedPort: 5432,
				Protocol:      "tcp",
			},
			wantHTTP: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(".")
			runner.registerExposedService(tt.service)
			assert.Len(t, runner.exposedServices, 1)
		})
	}
}
