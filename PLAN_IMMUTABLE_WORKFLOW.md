# Immutable /workflow Mount - Implementation Plan

## Overview

This document describes the implementation plan for making the `/workflow` mount immutable by default using a FUSE-based copy-on-write (CoW) filesystem. This prevents containers from accidentally or maliciously modifying files on the host system.

**Prerequisite**: This feature builds on top of the Volume Mounts feature (see `PLAN_VOLUME_MOUNTS.md`). The volume mounts provide explicit, controlled access to host directories when needed, while this feature makes the default `/workflow` mount safe by default.

## Core Concept

The `/workflow` mount should use a FUSE-based copy-on-write filesystem that:

- **Reads** are served directly from the original host filesystem (the workflow directory)
- **Writes** are redirected to a separate temporary overlay layer that exists only for the duration of the workflow execution
- Changes made by containers are visible within the workflow but never persist to the host
- The overlay is cleaned up when the workflow completes

This provides security by default - containers cannot modify real files even if they try.

```
Container sees:        /workflow
                           |
                     [FUSE CoW FS]
                      /         \
              [Overlay]        [Source]
           (temp writes)    (real files, ro)
                 |               |
            ~/.ocw/tmp/      ./my-project/
```

---

## FUSE Filesystem Implementation

### File: `pkg/fuse/cowfs.go` (NEW FILE)

Create a new package for the copy-on-write FUSE filesystem:

```go
package fuse

import (
    "context"
    "os"
    "path/filepath"
    "sync"

    "github.com/hanwen/go-fuse/v2/fs"
    "github.com/hanwen/go-fuse/v2/fuse"
)

// CowFS implements a copy-on-write filesystem
// Reads come from the source directory, writes go to the overlay directory
type CowFS struct {
    fs.Inode
    
    // Source directory (read-only, real files)
    SourceDir string
    
    // Overlay directory (read-write, temporary)
    OverlayDir string
    
    // Tracks which files have been copied to overlay
    modified map[string]bool
    mu       sync.RWMutex
}

// NewCowFS creates a new copy-on-write filesystem
func NewCowFS(sourceDir, overlayDir string) (*CowFS, error) {
    // Ensure directories exist
    if err := os.MkdirAll(overlayDir, 0755); err != nil {
        return nil, err
    }
    
    return &CowFS{
        SourceDir:  sourceDir,
        OverlayDir: overlayDir,
        modified:   make(map[string]bool),
    }, nil
}

// Mount mounts the FUSE filesystem at the given path
func (c *CowFS) Mount(mountPoint string) (*fuse.Server, error) {
    opts := &fs.Options{
        MountOptions: fuse.MountOptions{
            AllowOther: true,  // Allow container processes to access
            FsName:     "ocw-cow",
            Name:       "ocw",
        },
    }
    
    server, err := fs.Mount(mountPoint, c, opts)
    if err != nil {
        return nil, err
    }
    
    return server, nil
}

// resolvePath determines whether to read from source or overlay
func (c *CowFS) resolvePath(name string) string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    // Check if file exists in overlay (was modified)
    overlayPath := filepath.Join(c.OverlayDir, name)
    if _, err := os.Stat(overlayPath); err == nil {
        return overlayPath
    }
    
    // Fall back to source
    return filepath.Join(c.SourceDir, name)
}

// copyToOverlay copies a file from source to overlay for modification
func (c *CowFS) copyToOverlay(name string) (string, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    overlayPath := filepath.Join(c.OverlayDir, name)
    
    // Create parent directories in overlay
    if err := os.MkdirAll(filepath.Dir(overlayPath), 0755); err != nil {
        return "", err
    }
    
    // Check if source exists
    sourcePath := filepath.Join(c.SourceDir, name)
    if info, err := os.Stat(sourcePath); err == nil {
        // Copy source file to overlay
        data, err := os.ReadFile(sourcePath)
        if err != nil {
            return "", err
        }
        if err := os.WriteFile(overlayPath, data, info.Mode()); err != nil {
            return "", err
        }
    }
    
    c.modified[name] = true
    return overlayPath, nil
}

// Cleanup removes the overlay directory
func (c *CowFS) Cleanup() error {
    return os.RemoveAll(c.OverlayDir)
}
```

### File: `pkg/fuse/node.go` (NEW FILE)

FUSE node implementation with all required interfaces:

```go
package fuse

import (
    "context"
    "os"
    "path/filepath"
    "syscall"

    "github.com/hanwen/go-fuse/v2/fs"
    "github.com/hanwen/go-fuse/v2/fuse"
)

// CowNode represents a file or directory in the CoW filesystem
type CowNode struct {
    fs.Inode
    cow  *CowFS
    path string  // Relative path from root
}

// Ensure CowNode implements required interfaces
var _ = (fs.NodeLookuper)((*CowNode)(nil))
var _ = (fs.NodeReaddirer)((*CowNode)(nil))
var _ = (fs.NodeOpener)((*CowNode)(nil))
var _ = (fs.NodeCreater)((*CowNode)(nil))
var _ = (fs.NodeMkdirer)((*CowNode)(nil))
var _ = (fs.NodeUnlinker)((*CowNode)(nil))
var _ = (fs.NodeRmdirer)((*CowNode)(nil))
var _ = (fs.NodeRenamer)((*CowNode)(nil))
var _ = (fs.NodeGetattrer)((*CowNode)(nil))
var _ = (fs.NodeSetattrer)((*CowNode)(nil))

// Lookup looks up a child node
func (n *CowNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
    childPath := filepath.Join(n.path, name)
    realPath := n.cow.resolvePath(childPath)
    
    info, err := os.Lstat(realPath)
    if err != nil {
        return nil, syscall.ENOENT
    }
    
    child := &CowNode{cow: n.cow, path: childPath}
    
    // Set attributes
    out.Attr.Mode = uint32(info.Mode())
    out.Attr.Size = uint64(info.Size())
    out.Attr.Mtime = uint64(info.ModTime().Unix())
    
    stable := fs.StableAttr{Mode: info.Mode()}
    return n.NewInode(ctx, child, stable), 0
}

// Readdir reads directory contents
func (n *CowNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
    // Merge entries from source and overlay
    entries := make(map[string]fuse.DirEntry)
    
    // Read from source first
    sourcePath := filepath.Join(n.cow.SourceDir, n.path)
    if dirEntries, err := os.ReadDir(sourcePath); err == nil {
        for _, e := range dirEntries {
            info, _ := e.Info()
            entries[e.Name()] = fuse.DirEntry{
                Name: e.Name(),
                Mode: uint32(info.Mode()),
            }
        }
    }
    
    // Overlay entries take precedence
    overlayPath := filepath.Join(n.cow.OverlayDir, n.path)
    if dirEntries, err := os.ReadDir(overlayPath); err == nil {
        for _, e := range dirEntries {
            // Skip whiteout files in listing but use them to hide source files
            if isWhiteout(e.Name()) {
                originalName := getWhiteoutTarget(e.Name())
                delete(entries, originalName)
                continue
            }
            info, _ := e.Info()
            entries[e.Name()] = fuse.DirEntry{
                Name: e.Name(),
                Mode: uint32(info.Mode()),
            }
        }
    }
    
    // Convert map to slice
    result := make([]fuse.DirEntry, 0, len(entries))
    for _, e := range entries {
        result = append(result, e)
    }
    
    return fs.NewListDirStream(result), 0
}

// Open opens a file
func (n *CowNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
    realPath := n.cow.resolvePath(n.path)
    
    // If opening for write, copy to overlay first
    if flags&(syscall.O_WRONLY|syscall.O_RDWR|syscall.O_APPEND|syscall.O_TRUNC) != 0 {
        var err error
        realPath, err = n.cow.copyToOverlay(n.path)
        if err != nil {
            return nil, 0, syscall.EIO
        }
    }
    
    f, err := os.OpenFile(realPath, int(flags), 0)
    if err != nil {
        return nil, 0, syscall.EIO
    }
    
    return &cowFileHandle{file: f}, 0, 0
}

// Create creates a new file
func (n *CowNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
    childPath := filepath.Join(n.path, name)
    overlayPath := filepath.Join(n.cow.OverlayDir, childPath)
    
    // Ensure parent directory exists in overlay
    if err := os.MkdirAll(filepath.Dir(overlayPath), 0755); err != nil {
        return nil, nil, 0, syscall.EIO
    }
    
    f, err := os.OpenFile(overlayPath, int(flags)|os.O_CREATE, os.FileMode(mode))
    if err != nil {
        return nil, nil, 0, syscall.EIO
    }
    
    n.cow.mu.Lock()
    n.cow.modified[childPath] = true
    n.cow.mu.Unlock()
    
    child := &CowNode{cow: n.cow, path: childPath}
    stable := fs.StableAttr{Mode: os.FileMode(mode)}
    inode := n.NewInode(ctx, child, stable)
    
    info, _ := f.Stat()
    out.Attr.Mode = uint32(info.Mode())
    out.Attr.Size = uint64(info.Size())
    
    return inode, &cowFileHandle{file: f}, 0, 0
}

// Mkdir creates a directory
func (n *CowNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
    childPath := filepath.Join(n.path, name)
    overlayPath := filepath.Join(n.cow.OverlayDir, childPath)
    
    if err := os.MkdirAll(overlayPath, os.FileMode(mode)); err != nil {
        return nil, syscall.EIO
    }
    
    n.cow.mu.Lock()
    n.cow.modified[childPath] = true
    n.cow.mu.Unlock()
    
    child := &CowNode{cow: n.cow, path: childPath}
    stable := fs.StableAttr{Mode: os.ModeDir | os.FileMode(mode)}
    
    out.Attr.Mode = uint32(os.ModeDir | os.FileMode(mode))
    
    return n.NewInode(ctx, child, stable), 0
}

// Unlink removes a file
func (n *CowNode) Unlink(ctx context.Context, name string) syscall.Errno {
    childPath := filepath.Join(n.path, name)
    
    // Create whiteout in overlay to hide the file
    return n.createWhiteout(childPath)
}

// Rmdir removes a directory
func (n *CowNode) Rmdir(ctx context.Context, name string) syscall.Errno {
    childPath := filepath.Join(n.path, name)
    
    // Create whiteout in overlay to hide the directory
    return n.createWhiteout(childPath)
}

// createWhiteout creates a whiteout marker to indicate deletion
func (n *CowNode) createWhiteout(path string) syscall.Errno {
    whiteoutPath := filepath.Join(n.cow.OverlayDir, filepath.Dir(path), ".wh."+filepath.Base(path))
    
    // Ensure parent exists
    if err := os.MkdirAll(filepath.Dir(whiteoutPath), 0755); err != nil {
        return syscall.EIO
    }
    
    // Remove from overlay if it exists there
    overlayPath := filepath.Join(n.cow.OverlayDir, path)
    os.RemoveAll(overlayPath)
    
    // Create whiteout marker
    f, err := os.Create(whiteoutPath)
    if err != nil {
        return syscall.EIO
    }
    f.Close()
    
    return 0
}

// Rename renames a file or directory
func (n *CowNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
    oldPath := filepath.Join(n.path, name)
    
    newParentNode, ok := newParent.(*CowNode)
    if !ok {
        return syscall.EIO
    }
    newPath := filepath.Join(newParentNode.path, newName)
    
    // Copy source to overlay at new location
    sourcePath := n.cow.resolvePath(oldPath)
    newOverlayPath := filepath.Join(n.cow.OverlayDir, newPath)
    
    if err := os.MkdirAll(filepath.Dir(newOverlayPath), 0755); err != nil {
        return syscall.EIO
    }
    
    if err := os.Rename(sourcePath, newOverlayPath); err != nil {
        // If rename fails (cross-device), do copy + delete
        if err := copyFileOrDir(sourcePath, newOverlayPath); err != nil {
            return syscall.EIO
        }
    }
    
    // Create whiteout for old location
    n.createWhiteout(oldPath)
    
    n.cow.mu.Lock()
    n.cow.modified[newPath] = true
    n.cow.mu.Unlock()
    
    return 0
}

// Getattr gets file attributes
func (n *CowNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
    realPath := n.cow.resolvePath(n.path)
    
    info, err := os.Lstat(realPath)
    if err != nil {
        return syscall.ENOENT
    }
    
    out.Attr.Mode = uint32(info.Mode())
    out.Attr.Size = uint64(info.Size())
    out.Attr.Mtime = uint64(info.ModTime().Unix())
    out.Attr.Atime = uint64(info.ModTime().Unix())
    out.Attr.Ctime = uint64(info.ModTime().Unix())
    
    return 0
}

// Setattr sets file attributes
func (n *CowNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
    // Copy to overlay first for any attribute changes
    overlayPath, err := n.cow.copyToOverlay(n.path)
    if err != nil {
        return syscall.EIO
    }
    
    if in.Valid&fuse.FATTR_MODE != 0 {
        if err := os.Chmod(overlayPath, os.FileMode(in.Mode)); err != nil {
            return syscall.EIO
        }
    }
    
    if in.Valid&fuse.FATTR_SIZE != 0 {
        if err := os.Truncate(overlayPath, int64(in.Size)); err != nil {
            return syscall.EIO
        }
    }
    
    return n.Getattr(ctx, fh, out)
}

// cowFileHandle wraps an os.File
type cowFileHandle struct {
    file *os.File
}

func (h *cowFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
    n, err := h.file.ReadAt(dest, off)
    if err != nil && err.Error() != "EOF" {
        return nil, syscall.EIO
    }
    return fuse.ReadResultData(dest[:n]), 0
}

func (h *cowFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
    n, err := h.file.WriteAt(data, off)
    if err != nil {
        return 0, syscall.EIO
    }
    return uint32(n), 0
}

func (h *cowFileHandle) Release(ctx context.Context) syscall.Errno {
    h.file.Close()
    return 0
}

func (h *cowFileHandle) Flush(ctx context.Context) syscall.Errno {
    return 0
}

func (h *cowFileHandle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
    return syscall.Errno(syscall.Fsync(int(h.file.Fd())))
}

// Helper functions for whiteout handling
func isWhiteout(name string) bool {
    return len(name) > 4 && name[:4] == ".wh."
}

func getWhiteoutTarget(name string) string {
    return name[4:]
}

func copyFileOrDir(src, dst string) error {
    info, err := os.Stat(src)
    if err != nil {
        return err
    }
    
    if info.IsDir() {
        return copyDir(src, dst)
    }
    return copyFile(src, dst)
}

func copyFile(src, dst string) error {
    data, err := os.ReadFile(src)
    if err != nil {
        return err
    }
    info, err := os.Stat(src)
    if err != nil {
        return err
    }
    return os.WriteFile(dst, data, info.Mode())
}

func copyDir(src, dst string) error {
    if err := os.MkdirAll(dst, 0755); err != nil {
        return err
    }
    
    entries, err := os.ReadDir(src)
    if err != nil {
        return err
    }
    
    for _, e := range entries {
        srcPath := filepath.Join(src, e.Name())
        dstPath := filepath.Join(dst, e.Name())
        if err := copyFileOrDir(srcPath, dstPath); err != nil {
            return err
        }
    }
    
    return nil
}
```

### File: `pkg/fuse/manager.go` (NEW FILE)

Manager to handle FUSE mounts per workflow:

```go
package fuse

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"

    "github.com/hanwen/go-fuse/v2/fuse"
)

// MountManager manages FUSE mounts for a workflow execution
type MountManager struct {
    // Base directory for all temporary mounts and overlays
    BaseDir string
    
    // Active mounts
    mounts map[string]*MountInfo
    mu     sync.Mutex
}

// MountInfo contains information about an active mount
type MountInfo struct {
    SourceDir   string
    MountPoint  string
    OverlayDir  string
    Server      *fuse.Server
    CowFS       *CowFS
}

// NewMountManager creates a new mount manager
func NewMountManager(workflowRunID string) (*MountManager, error) {
    baseDir := filepath.Join(os.TempDir(), "ocw-mounts", workflowRunID)
    if err := os.MkdirAll(baseDir, 0755); err != nil {
        return nil, err
    }
    
    return &MountManager{
        BaseDir: baseDir,
        mounts:  make(map[string]*MountInfo),
    }, nil
}

// CreateCowMount creates a copy-on-write mount for a source directory
func (m *MountManager) CreateCowMount(name, sourceDir string) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    mountPoint := filepath.Join(m.BaseDir, "mnt", name)
    overlayDir := filepath.Join(m.BaseDir, "overlay", name)
    
    if err := os.MkdirAll(mountPoint, 0755); err != nil {
        return "", err
    }
    
    cowfs, err := NewCowFS(sourceDir, overlayDir)
    if err != nil {
        return "", err
    }
    
    server, err := cowfs.Mount(mountPoint)
    if err != nil {
        return "", err
    }
    
    m.mounts[name] = &MountInfo{
        SourceDir:  sourceDir,
        MountPoint: mountPoint,
        OverlayDir: overlayDir,
        Server:     server,
        CowFS:      cowfs,
    }
    
    return mountPoint, nil
}

// GetMountPoint returns the mount point for a named mount
func (m *MountManager) GetMountPoint(name string) (string, bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    info, ok := m.mounts[name]
    if !ok {
        return "", false
    }
    return info.MountPoint, true
}

// Cleanup unmounts all FUSE filesystems and cleans up temporary directories
func (m *MountManager) Cleanup() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    var errs []error
    
    for name, mount := range m.mounts {
        if err := mount.Server.Unmount(); err != nil {
            errs = append(errs, fmt.Errorf("unmount %s: %w", name, err))
        }
        if err := mount.CowFS.Cleanup(); err != nil {
            errs = append(errs, fmt.Errorf("cleanup %s: %w", name, err))
        }
    }
    
    // Remove base directory
    if err := os.RemoveAll(m.BaseDir); err != nil {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("cleanup errors: %v", errs)
    }
    return nil
}
```

### File: `pkg/fuse/fuse.go` (NEW FILE)

Platform detection and availability checking:

```go
package fuse

import (
    "os/exec"
    "runtime"
)

// Available checks if FUSE is available on this system
func Available() bool {
    switch runtime.GOOS {
    case "linux":
        return checkLinuxFuse()
    case "darwin":
        return checkMacFuse()
    default:
        return false
    }
}

func checkLinuxFuse() bool {
    // Check if /dev/fuse exists
    if _, err := exec.LookPath("fusermount"); err != nil {
        return false
    }
    return true
}

func checkMacFuse() bool {
    // Check for macFUSE or FUSE-T
    // macFUSE: /Library/Filesystems/macfuse.fs
    // FUSE-T: /Library/Filesystems/fuse-t.fs
    paths := []string{
        "/Library/Filesystems/macfuse.fs",
        "/Library/Filesystems/fuse-t.fs",
        "/usr/local/lib/libfuse.dylib",
    }
    
    for _, p := range paths {
        if fileExists(p) {
            return true
        }
    }
    
    return false
}

func fileExists(path string) bool {
    _, err := exec.LookPath(path)
    return err == nil
}
```

---

## Runner Changes

### File: `pkg/runner/runner.go`

#### Add MountManager to Runner

```go
type Runner struct {
    // ... existing fields ...
    
    // NEW: FUSE mount manager for copy-on-write filesystem
    mountManager    *fuse.MountManager
    
    // NEW: Whether FUSE is being used (false = fallback mode)
    useFuse         bool
    
    // NEW: The mount point for /workflow (either FUSE mount or real dir)
    workflowMountPoint string
}
```

#### Initialize MountManager in Run()

```go
func (r *Runner) Run(ctx context.Context, cfg RunConfig) error {
    // ... existing setup code ...
    
    // Initialize FUSE mount for workflow directory
    if fuse.Available() && !cfg.NoFuse {
        r.mountManager, err = fuse.NewMountManager(r.runID)
        if err != nil {
            r.Output(r.styles.Warning("Failed to create mount manager, falling back to direct mounts: %v", err))
            r.useFuse = false
            r.workflowMountPoint = r.WorkflowDir
        } else {
            // Create CoW mount for workflow directory
            mountPoint, err := r.mountManager.CreateCowMount("workflow", r.WorkflowDir)
            if err != nil {
                r.Output(r.styles.Warning("Failed to create CoW mount, falling back to direct mounts: %v", err))
                r.mountManager.Cleanup()
                r.mountManager = nil
                r.useFuse = false
                r.workflowMountPoint = r.WorkflowDir
            } else {
                r.useFuse = true
                r.workflowMountPoint = mountPoint
            }
        }
    } else {
        if !fuse.Available() {
            r.Output(r.styles.Warning("FUSE not available, using direct mounts (containers can modify host files)"))
        }
        r.useFuse = false
        r.workflowMountPoint = r.WorkflowDir
    }
    
    // Ensure cleanup on exit
    defer func() {
        if r.mountManager != nil {
            if err := r.mountManager.Cleanup(); err != nil {
                r.Output(r.styles.Warning("Failed to cleanup mounts: %v", err))
            }
        }
    }()
    
    // ... rest of Run() ...
}
```

#### Update Container Execution

When running containers, use `r.workflowMountPoint` instead of `r.WorkflowDir`:

```go
// In runStep or wherever containers are executed
opts := RunContainerOptions{
    // ... other options ...
    WorkflowDir: r.workflowMountPoint,  // Use FUSE mount point if available
}
```

### File: `pkg/runner/podman.go`

No changes needed - the `WorkflowDir` field already accepts a path. When FUSE is active, it will receive the FUSE mount point path instead of the real directory.

---

## CLI Changes

### Add --no-fuse Flag

Add a flag to disable FUSE for debugging or when FUSE causes issues:

```go
// In cmd/run.go or wherever CLI flags are defined
runCmd.Flags().Bool("no-fuse", false, "Disable FUSE filesystem isolation (allows containers to modify host files)")
```

### RunConfig Update

```go
type RunConfig struct {
    // ... existing fields ...
    
    // NoFuse disables FUSE-based filesystem isolation
    NoFuse bool
}
```

---

## Platform Considerations

### macOS

- Requires macFUSE (https://osxfuse.github.io/) or FUSE-T (https://github.com/macos-fuse-t/fuse-t)
- Users may need to:
  1. Install macFUSE or FUSE-T
  2. Grant permissions in System Preferences > Security & Privacy
  3. Reboot after installation
- Consider providing helpful error message with installation instructions

### Linux

- FUSE is typically available but may require:
  1. `fuse` package installation: `apt install fuse` or `yum install fuse`
  2. User added to `fuse` group: `usermod -aG fuse $USER`
  3. `/etc/fuse.conf` may need `user_allow_other` for `AllowOther` option

### Fallback Mode

If FUSE is not available or fails:
1. Log a warning about reduced security
2. Fall back to direct bind mounts (current behavior)
3. Workflows continue to work, just without isolation

```
WARNING: FUSE not available, using direct mounts (containers can modify host files)
```

---

## Security Considerations

1. **Default Deny**: By default, containers cannot modify host files in `/workflow`
2. **Explicit Access**: Real write access requires explicit volume declarations (see PLAN_VOLUME_MOUNTS.md)
3. **Overlay Cleanup**: Overlay directories are cleaned up after workflow completion
4. **No Symlink Escape**: FUSE implementation should not follow symlinks that point outside the source directory
5. **Graceful Degradation**: Falls back to direct mounts if FUSE unavailable (with warning)

---

## Testing Strategy

### Unit Tests
- FUSE availability detection
- Mount manager lifecycle
- CoW filesystem operations:
  - Read from source
  - Write creates overlay copy
  - Directory listing merges source + overlay
  - Delete creates whiteout
  - Rename handled correctly

### Integration Tests
- Workflow runs with FUSE enabled
- Container writes don't affect host
- Container can read all source files
- Cleanup removes all temporary files
- Fallback mode when FUSE unavailable
- `--no-fuse` flag works

### Platform Tests
- macOS with macFUSE
- macOS with FUSE-T
- Linux with FUSE
- Fallback behavior on systems without FUSE

---

## Implementation Order

### Phase 1: FUSE Package
1. Add `github.com/hanwen/go-fuse/v2` dependency to `go.mod`
2. Create `pkg/fuse/fuse.go` with availability detection
3. Create `pkg/fuse/cowfs.go` with basic CoW implementation
4. Create `pkg/fuse/node.go` with FUSE node interfaces
5. Create `pkg/fuse/manager.go` for mount management
6. Write unit tests for FUSE operations

### Phase 2: Runner Integration
1. Add `mountManager`, `useFuse`, `workflowMountPoint` to Runner
2. Initialize FUSE in `Run()` with fallback handling
3. Pass `workflowMountPoint` to container execution
4. Ensure proper cleanup on workflow completion/error

### Phase 3: CLI Integration
1. Add `--no-fuse` flag
2. Add RunConfig.NoFuse field
3. Wire flag to runner

### Phase 4: Polish
1. Helpful error messages for FUSE installation
2. Documentation
3. Platform-specific testing

---

## Dependencies to Add

```go
// go.mod additions
require (
    github.com/hanwen/go-fuse/v2 v2.5.0
)
```

---

## File Summary

| File | Action | Description |
|------|--------|-------------|
| `pkg/fuse/fuse.go` | Create | Platform detection and availability |
| `pkg/fuse/cowfs.go` | Create | Copy-on-write FUSE filesystem |
| `pkg/fuse/node.go` | Create | FUSE node implementations |
| `pkg/fuse/manager.go` | Create | Mount lifecycle management |
| `pkg/runner/runner.go` | Modify | FUSE initialization and usage |
| `cmd/run.go` | Modify | Add --no-fuse flag |
| `go.mod` | Modify | Add go-fuse dependency |

---

## Open Questions

1. **Watch mode**: How should file watching interact with CoW? The watcher currently watches the real directory. Should it also watch the overlay for changes made by containers?

2. **Build step context**: Should the build context also use CoW? Builds often need to write intermediate files. Using CoW might slow down builds.

3. **Performance**: For large directories, CoW may have performance implications. Should there be a size threshold or configuration option?

4. **Outputs directory**: The `.ocw-outputs` directory is currently in the workflow directory. Should container outputs persist after CoW cleanup?
