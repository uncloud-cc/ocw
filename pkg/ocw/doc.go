// Package ocw provides the core interfaces and types for implementing
// OCW workflow execution engines.
//
// This package defines:
//
//   - Step: represents a single unit of work in an OCW workflow with its
//     execution lifecycle, inputs, outputs, and child steps
//
//   - StepExecutor: executes steps and returns results
//
//   - ExecutionContext: provides runtime context for step execution
//
// Container runtime interfaces are defined in the container subpackage:
//
//   - container.Runtime: unified interface for all container operations
//   - container.Runner: container execution (docker run)
//   - container.Builder: image building (docker build)
//   - container.Registry: push/pull operations
//   - container.NetworkManager: network creation and management
//   - container.VolumeManager: volume creation and management
//
// These interfaces allow OCW to run workflows in various execution contexts
// without coupling to specific container technologies.
package ocw
