# Security Model

This document describes the security architecture of OCW (Open Container Workflows), with a focus on how workflow containers are isolated from the host system.

## Overview

OCW executes workflow steps inside containers using Podman. A core security principle is that **workflow code should not be able to modify the host filesystem** unless explicitly permitted through volume mounts.

This protects against:
- **Malicious workflow code** - Untrusted scripts cannot write malware to your system
- **Compromised container images** - Images from public registries cannot persist malicious files
- **Supply chain attacks** - Dependencies installed during builds cannot modify your source code

## Immutable Filesystem

By default, OCW mounts your workflow directory into containers using an **immutable copy-on-write filesystem**. This means:

| Operation | Behavior |
|-----------|----------|
| **Read files** | Container sees your actual workflow files |
| **Write files** | Writes go to an ephemeral overlay layer |
| **Modify files** | Modifications are stored in the overlay, original unchanged |
| **Delete files** | Deletions are recorded in the overlay, original unchanged |

### How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                     Your Host System                         │
│                                                              │
│   /path/to/project/        (your source code - PROTECTED)   │
│   ├── src/                                                   │
│   ├── package.json                                           │
│   └── ...                                                    │
│                                                              │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               │ Mounted as READ-ONLY lower layer
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                    Overlay Filesystem                        │
│                                                              │
│   Lower layer:  Your project files (read-only)              │
│   Upper layer:  Ephemeral storage (container writes here)   │
│                                                              │
│   Container sees a merged view where it appears writable    │
│   but all writes are captured in the ephemeral upper layer  │
│                                                              │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               │ Mounted at /workflow
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                       Container                              │
│                                                              │
│   /workflow/               (appears as normal directory)    │
│   ├── src/                 (from host - read works)         │
│   ├── package.json         (from host - read works)         │
│   ├── node_modules/        (written by npm install)         │
│   └── dist/                (written by build step)          │
│                                                              │
│   The container can read, write, and delete freely.         │
│   All changes are isolated to the ephemeral overlay.        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Step-to-Step File Sharing

Workflow steps can share files with each other through cascading overlays:

1. **Step 1** runs: writes go to `overlay-step-1`
2. **Step 2** runs: sees `overlay-step-1` + original files, writes go to `overlay-step-2`
3. **Step 3** runs: sees `overlay-step-2` + `overlay-step-1` + original files

This allows build artifacts to flow between steps (e.g., `npm install` creates `node_modules/` that `npm build` can use) while keeping your host filesystem untouched.

### Cleanup

When a workflow job completes, all overlay layers are destroyed. No workflow-generated files persist to your host system unless you explicitly configure volume mounts.

## Rootless Containers

OCW uses **rootless Podman** by default. This means:

- Podman runs as your regular user, not as root
- Containers run inside a **user namespace** where UID 0 inside the container maps to your UID outside
- Even if a container process is "root" inside, it has no special privileges on the host
- Kernel capabilities are restricted to what's safe for unprivileged users

### What Rootless Protects Against

| Threat | Protection |
|--------|------------|
| Container escape to host root | Container root = your user, not real root |
| Accessing other users' files | User namespace prevents access |
| Modifying system files | No write access outside namespace |
| Loading kernel modules | No CAP_SYS_MODULE capability |
| Accessing raw devices | No device access by default |

## Network Isolation

Each workflow job runs in its own isolated network:

- Containers can communicate with each other within the job
- Containers can access the internet (for package downloads, etc.)
- External systems cannot directly connect to workflow containers (unless ports are explicitly exposed)

## Explicit Volume Mounts

> **Important Disclaimer**: The immutable filesystem protections described above **do not apply** to explicit volume mounts.

Volume mounts are designed as conscious entrypoints to interact with the host filesystem when needed. If you configure a volume mount, you are explicitly granting the container write access to that path.

### Example: Volume Mount Bypasses Immutability

```yaml
name: my-workflow
volumes:
  output:
    path: ./dist          # Host path
    mountPath: /output    # Container path
    mode: readwrite       # Explicit write permission

jobs:
  build:
    steps:
      - name: build
        image: node:20
        volumes:
          - output         # This volume CAN write to host ./dist
        cmd: |
          npm run build
          cp -r dist/* /output/   # This WILL modify host filesystem
```

In this example:
- `/workflow` is still immutable (writes to overlay)
- `/output` is a direct mount to `./dist` on the host (writes persist)

### Volume Mount Security Recommendations

1. **Use read-only mounts when possible**
   ```yaml
   volumes:
     config:
       path: ./config
       mode: readonly    # Container cannot modify
   ```

2. **Limit write mounts to specific directories**
   - Don't mount your entire project as writable
   - Create dedicated output directories for build artifacts

3. **Review volume mounts in untrusted workflows**
   - Before running a workflow from an external source, check what volumes it requests
   - Be suspicious of write mounts to sensitive paths

## Secret Handling

OCW masks secret values in output to prevent accidental exposure:

```yaml
env:
  API_KEY:
    secret: true
    value: $API_KEY
```

When a secret value appears in container output, it's replaced with `[secret]`.

**Note**: This is output masking only. The actual secret value is still passed to the container as an environment variable. Containers have full access to their environment variables.

### Secret Recommendations

1. **Use environment variables or secret managers** - Don't hardcode secrets in workflow files
2. **Don't echo secrets** - Avoid `echo $SECRET` in scripts
3. **Be cautious with verbose logging** - Build tools may log environment variables

## Container Image Security

OCW pulls container images from registries. The security of your workflow depends partly on the trustworthiness of these images.

### Recommendations

1. **Use specific tags, not `latest`**
   ```yaml
   image: node:20.10.0    # Good: specific version
   image: node:latest     # Risky: could change unexpectedly
   ```

2. **Prefer official images** - Docker Official Images and Verified Publishers are more trustworthy

3. **Consider image scanning** - Use tools like `podman scan` or Trivy to check for vulnerabilities

4. **Use digest pinning for critical workflows**
   ```yaml
   image: node@sha256:abc123...    # Immutable reference
   ```

## Threat Model Summary

| Threat | Mitigation | Residual Risk |
|--------|------------|---------------|
| Malicious workflow modifies host files | Immutable overlay filesystem | Explicit volume mounts bypass this |
| Container escape via capabilities | Capability dropping (CAP_DROP=ALL) | None - only CHOWN/SETUID/SETGID allowed |
| Privilege escalation via SUID | No-new-privileges flag | None - SUID binaries cannot escalate |
| Dangerous syscall exploits | Custom seccomp profile | Kernel 0-days could still exploit |
| Container escape (unprivileged) | User namespace isolation | Kernel vulnerabilities could allow escape |
| Container escape (privileged) | Rootless Podman + capability dropping | N/A - no privileged containers |
| Malicious container image | User namespace limits damage | Image could still exfiltrate data |
| Network-based attacks from container | Isolated network namespace | Outbound connections allowed |
| Secret exposure | Output masking | Secrets accessible inside container |
| Supply chain (dependencies) | Immutable FS prevents persistence | Malicious code runs during build |

## Jailbreak Prevention (Implemented)

OCW implements multiple layers of jailbreak prevention:

### 1. Capability Dropping

**Status:** ✅ **Implemented**

All containers run with `--cap-drop=ALL` followed by minimal capability additions:
- `CHOWN` - Required for file ownership changes
- `SETGID` - Required for group ID changes  
- `SETUID` - Required for user ID changes

This prevents containers from:
- Loading kernel modules (`CAP_SYS_MODULE`)
- Modifying network configuration (`CAP_NET_ADMIN`)
- Changing system time (`CAP_SYS_TIME`)
- Accessing raw network packets (`CAP_NET_RAW`)
- Performing privileged operations

### 2. No-New-Privileges

**Status:** ✅ **Implemented**

All containers run with `--security-opt no-new-privileges:true`

This prevents:
- SUID binaries from escalating privileges
- Setuid/setgid bits from granting additional capabilities
- Exploitation of setuid root binaries inside containers

**Example:** Even if a container has a vulnerable `sudo` binary, it cannot escalate to real root.

### 3. Seccomp Profile

**Status:** ✅ **Implemented**

OCW uses a custom seccomp profile that blocks dangerous syscalls:

**Blocked Syscalls:**
- `mount`, `umount`, `umount2` - Prevent filesystem mounting/escapes
- `pivot_root` - Prevent chroot escape attacks  
- `chroot` - Prevent chroot jailbreaks
- `ptrace` - Prevent process injection attacks
- `bpf` - Prevent BPF program loading (kernel exploitation)
- `personality` - Prevent personality syscall attacks
- `process_vm_readv/writev` - Prevent cross-process memory access
- `init_module`, `delete_module` - Prevent kernel module loading
- Various kernel-related syscalls

**Allowed:** Standard container operations (file I/O, networking, process management, etc.)

### Defense in Depth Summary

```
┌─────────────────────────────────────────────────────────────┐
│                    Jailbreak Prevention                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Layer 1: Immutable Filesystem                              │
│  ├── Overlay volumes (read-only lower layer)                │
│  └── Host files protected from modification                 │
│                                                             │
│  Layer 2: Rootless Podman                                   │
│  ├── User namespace isolation                               │
│  └── Container root ≠ Host root                             │
│                                                             │
│  Layer 3: Capability Dropping                               │
│  ├── --cap-drop=ALL                                         │
│  └── Only CHOWN/SETUID/SETGID allowed                       │
│                                                             │
│  Layer 4: No-New-Privileges                                  │
│  ├── --security-opt no-new-privileges:true                  │
│  └── Prevents SUID escalation                               │
│                                                             │
│  Layer 5: Seccomp Profile                                   │
│  ├── Blocks dangerous syscalls                              │
│  └── mount, pivot_root, ptrace, bpf, etc.                   │
│                                                             │
│  Layer 6: Network Isolation                                 │
│  ├── Per-job bridge networks                                │
│  └── Selective port exposure only                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Security Configuration

### Immutable Filesystem (Always Enabled)

The immutable filesystem is **always active** and cannot be disabled. This ensures that:
- Your host filesystem is always protected from container writes
- Workflow steps can share files via cascading overlays
- All container modifications are ephemeral

If you need to persist files to the host filesystem, use explicit volume mounts as described in the [Explicit Volume Mounts](#explicit-volume-mounts) section.

### SELinux Considerations

On systems with SELinux (Fedora, RHEL, CentOS), OCW uses `--security-opt label=disable` for containers using overlay volumes. This is required for the overlay mount to be accessible but disables SELinux label enforcement for that container.

If you require SELinux enforcement, you can:
1. Create a custom SELinux policy for OCW containers
2. Review OCW's security settings and consider if the trade-off is acceptable for your use case

## Reporting Security Issues

If you discover a security vulnerability in OCW, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email security concerns to the maintainers directly
3. Allow reasonable time for a fix before public disclosure

## Further Reading

- [Podman Rootless Documentation](https://github.com/containers/podman/blob/main/rootless.md)
- [Linux User Namespaces](https://man7.org/linux/man-pages/man7/user_namespaces.7.html)
- [Overlay Filesystem](https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt)
- [Container Security Best Practices](https://cheatsheetseries.owasp.org/cheatsheets/Container_Security_Cheat_Sheet.html)
- Loading kernel modules (CAP_SYS_MODULE)
- Modifying network config (CAP_NET_ADMIN)
- Changing system time (CAP_SYS_TIME)
- Accessing raw network packets (CAP_NET_RAW)

### 2. Seccomp Profiles

**Current Status:** Default seccomp profile (allows most safe syscalls)

**Recommended Addition:** Custom seccomp profile to restrict:
- mount() syscalls (prevents mount namespace escape)
- pivot_root() (prevents chroot escape)
- ptrace() (prevents process injection)
- unnecessary socket operations

### 3. Read-Only Root Filesystem

**Current Status:** Container rootfs is writable

**Recommended Addition:** `--read-only` flag to make the container's root filesystem read-only, forcing all writes to explicitly mounted volumes.

### 4. No New Privileges

**Current Status:** Not set

**Recommended Addition:** `--security-opt no-new-privileges:true` to prevent processes from gaining additional privileges via SUID binaries.

### 5. Resource Limits

**Current Status:** No CPU/memory limits enforced

**Recommended Addition:** Default resource limits:
```bash
podman run --memory=2g --cpus=2 --pids-limit=1024 ...
```

This prevents:
- Denial of service via memory exhaustion
- CPU abuse (cryptocurrency mining)
- Fork bombs (PID exhaustion)

### 6. Device Restrictions

**Current Status:** Default device access

**Recommended Addition:** Explicit device control:
```bash
podman run --device=/dev/null --device=/dev/zero --device=/dev/random --device=/dev/urandom ...
```

Prevents access to:
- Physical storage devices
- Kernel interfaces (/dev/kmsg, /dev/kmem)
- Hardware devices that could be exploited

### 7. User Namespace Isolation

**Current Status:** Relies on default rootless mapping

**Recommended Addition:** Explicit user namespace configuration:
```bash
podman run --userns=auto --uidmap=0:100000:65536 ...
```

Provides stronger isolation between containers and host.

### Implementation Priority

| Hardening Measure | Security Benefit | Implementation Complexity | Priority |
|-------------------|------------------|---------------------------|----------|
| Capability dropping | High | Low | **High** |
| No-new-privileges | High | Low | **High** |
| Resource limits | Medium | Low | Medium |
| Read-only rootfs | Medium | Medium | Medium |
| Seccomp profiles | High | High | Medium |
| Device restrictions | Medium | Low | Medium |
| User namespace maps | Medium | Medium | Low |

### Current vs. Hardened Comparison

| Threat | Current Protection | With Additional Hardening |
|--------|-------------------|---------------------------|
| Container escape via capabilities | Default rootless (good) | Explicit minimal caps (excellent) |
| Kernel exploit via syscalls | Default seccomp (good) | Custom seccomp (better) |
| SUID privilege escalation | None | No-new-privileges blocks |
| Resource exhaustion | None | Resource limits enforced |
| Device-based attacks | Default (some protection) | Explicit device allowlist |
