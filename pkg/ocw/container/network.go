package container

import (
	"context"
)

// NetworkManager creates and manages container networks.
type NetworkManager interface {
	// Create creates an isolated network for containers to communicate.
	Create(ctx context.Context, opts NetworkOptions) (Network, error)

	// Get retrieves an existing network by name or ID.
	Get(ctx context.Context, nameOrID string) (Network, error)

	// List returns networks matching the given filters.
	List(ctx context.Context, opts NetworkListOptions) ([]Network, error)
}

// Network represents an isolated network for container communication.
type Network interface {
	// ID returns the unique identifier of this network.
	ID() string

	// Name returns the human-readable name of this network.
	Name() string

	// Connect attaches a container to this network.
	Connect(ctx context.Context, containerID string, opts NetworkConnectOptions) error

	// Disconnect removes a container from this network.
	Disconnect(ctx context.Context, containerID string) error

	// Remove deletes this network. All containers must be disconnected first.
	Remove(ctx context.Context) error

	// Inspect returns detailed information about the network.
	Inspect(ctx context.Context) (NetworkInfo, error)
}

// NetworkOptions configures network creation.
type NetworkOptions struct {
	// Name is the network name (required).
	Name string

	// Driver is the network driver (e.g., "bridge", "overlay", "host", "none").
	Driver string

	// Internal makes the network isolated from external access.
	Internal bool

	// EnableIPv6 enables IPv6 on the network.
	EnableIPv6 bool

	// Subnet is the subnet in CIDR format (e.g., "172.18.0.0/16").
	Subnet string

	// Gateway is the gateway address for the subnet.
	Gateway string

	// Labels are metadata key-value pairs.
	Labels map[string]string
}

// NetworkListOptions configures network listing.
type NetworkListOptions struct {
	// Filters restricts the list to networks matching these criteria.
	// Common filters: "name", "id", "driver", "label", "scope".
	Filters map[string][]string
}

// NetworkConnectOptions configures how a container joins a network.
type NetworkConnectOptions struct {
	// Aliases are DNS names for this container on the network.
	Aliases []string

	// IPAddress assigns a specific IP address.
	IPAddress string

	// IPv6Address assigns a specific IPv6 address.
	IPv6Address string
}

// NetworkInfo contains detailed network metadata.
type NetworkInfo struct {
	// ID is the network identifier.
	ID string

	// Name is the network name.
	Name string

	// Driver is the network driver.
	Driver string

	// Scope is the network scope (local, swarm, global).
	Scope string

	// Internal indicates if the network is isolated.
	Internal bool

	// EnableIPv6 indicates if IPv6 is enabled.
	EnableIPv6 bool

	// Subnet is the network subnet.
	Subnet string

	// Gateway is the network gateway.
	Gateway string

	// Containers lists containers attached to this network.
	Containers map[string]NetworkContainerInfo

	// Labels are the network's metadata.
	Labels map[string]string
}

// NetworkContainerInfo contains info about a container on a network.
type NetworkContainerInfo struct {
	// Name is the container name.
	Name string

	// IPv4Address is the container's IPv4 address on this network.
	IPv4Address string

	// IPv6Address is the container's IPv6 address on this network.
	IPv6Address string

	// Aliases are the container's DNS aliases on this network.
	Aliases []string
}
