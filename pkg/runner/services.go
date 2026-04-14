package runner

import (
	"fmt"
)

// ExposedService tracks a service that has been exposed to the host
type ExposedService struct {
	StepID        string // ID of the step (used as identifier)
	StepName      string // Human-readable name of the step
	ContainerPort int    // Port inside the container
	HostPort      int    // Port on the host (may differ if preferred port was unavailable)
	RequestedPort int    // Originally requested host port
	Protocol      string // Protocol (http, https, tcp, udp)
}

// registerExposedService adds a service to the exposed services list
func (r *Runner) registerExposedService(svc ExposedService) {
	r.exposedMu.Lock()
	defer r.exposedMu.Unlock()
	r.exposedServices = append(r.exposedServices, svc)
}

// printExposedServices prints a summary of all exposed services
func (r *Runner) printExposedServices() {
	r.exposedMu.Lock()
	services := make([]ExposedService, len(r.exposedServices))
	copy(services, r.exposedServices)
	r.exposedMu.Unlock()

	if len(services) == 0 {
		return
	}

	r.Output("\n")
	r.Output(r.styles.Header("  Exposed Services"))
	r.Output("\n")
	r.Output(r.styles.Divider(40))
	r.Output("\n")

	for _, svc := range services {
		// Format the URL based on protocol
		var url string
		switch svc.Protocol {
		case "http":
			url = fmt.Sprintf("http://localhost:%d", svc.HostPort)
		case "https":
			url = fmt.Sprintf("https://localhost:%d", svc.HostPort)
		default:
			// For tcp, udp, etc. just show host:port
			url = fmt.Sprintf("localhost:%d", svc.HostPort)
		}

		// Show identifier (prefer ID, fall back to name)
		identifier := svc.StepID
		if identifier == "" {
			identifier = svc.StepName
		}

		// Show if port was reassigned
		if svc.HostPort != svc.RequestedPort {
			r.Output(r.styles.ServiceURL(identifier, url, fmt.Sprintf("%s, requested: %d", svc.Protocol, svc.RequestedPort)))
		} else {
			r.Output(r.styles.ServiceURL(identifier, url, svc.Protocol))
		}
	}
}
