package ocw

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorkflowRef represents a parsed remote workflow reference.
//
// Supported formats:
//
//	github.com/owner/repo                          → default file, branch "main"
//	github.com/owner/repo/workflow.yaml            → explicit file, branch "main"
//	github.com/owner/repo#my-branch                → default file, branch "my-branch"
//	github.com/owner/repo/workflow.yaml#my-branch  → explicit file, branch
//	github.com/owner/repo@v1.0.0                   → default file, tag "v1.0.0"
//	github.com/owner/repo/workflow.yaml@v1.0.0     → explicit file, tag
//	owner/repo                                     → defaults host to github.com
//	github.com/owner/repo@abc123...                → default file, explicit SHA
//	github.com/owner/repo/workflow.yaml@abc123...  → explicit file, explicit SHA
//
// The @ prefix means "this is a tag or SHA" and # means "this is a branch".
// If neither prefix is present, the default ref is the branch "main".
// If the ref is 40 or more hex characters, it is treated as a raw SHA.
type WorkflowRef struct {
	Host    string // e.g. "github.com"
	Owner   string // e.g. "uncloud-cc"
	Repo    string // e.g. "ocw"
	Path    string // file or directory path within repo (may be empty)
	Ref     string // e.g. "main", "v1.0.0", "abc123..."
	RefType string // "branch", "tag", or "sha"
}

// ParseWorkflowRef parses a raw reference string into a WorkflowRef.
// Local paths (starting with ".", "/", or "\") are rejected.
func ParseWorkflowRef(raw string) (*WorkflowRef, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("empty workflow reference")
	}

	// Reject obviously local paths
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "\\") {
		return nil, fmt.Errorf("local paths are not supported by ParseWorkflowRef: %q", raw)
	}

	// Extract ref suffix: # = branch, @ = tag
	var ref, refType string
	if idx := strings.LastIndex(s, "#"); idx >= 0 {
		ref = s[idx+1:]
		refType = "branch"
		s = s[:idx]
	} else if idx := strings.LastIndex(s, "@"); idx >= 0 {
		ref = s[idx+1:]
		refType = "tag"
		s = s[:idx]
	}

	// Default to main branch
	if ref == "" {
		ref = "main"
		refType = "branch"
	}

	// Detect raw SHA: 40+ hex chars
	if refType == "tag" && looksLikeSHA(ref) {
		refType = "sha"
	}

	s = strings.TrimSuffix(s, "/")
	parts := strings.Split(s, "/")

	var host, owner, repo string
	var path string

	if len(parts) >= 3 && strings.Contains(parts[0], ".") {
		// Explicit host: github.com/owner/repo/...
		host = parts[0]
		owner = parts[1]
		repo = parts[2]
		if len(parts) > 3 {
			path = strings.Join(parts[3:], "/")
		}
	} else if len(parts) >= 2 {
		// No host: owner/repo/...  → default to github.com
		host = "github.com"
		owner = parts[0]
		repo = parts[1]
		if len(parts) > 2 {
			path = strings.Join(parts[2:], "/")
		}
	} else {
		return nil, fmt.Errorf("invalid workflow reference %q: expected at least owner/repo", raw)
	}

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid workflow reference %q: owner and repo are required", raw)
	}

	return &WorkflowRef{
		Host:    host,
		Owner:   owner,
		Repo:    repo,
		Path:    path,
		Ref:     ref,
		RefType: refType,
	}, nil
}

// URL returns the HTTPS URL for the repository.
func (r *WorkflowRef) URL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.Host, r.Owner, r.Repo)
}

// CachePath returns the local cache directory for this reference at the given SHA.
func (r *WorkflowRef) CachePath(baseDir, sha string) string {
	return filepath.Join(baseDir, r.Host, r.Owner, r.Repo, sha, "contents")
}

// MetaPath returns the path to the .meta file for a cached reference.
func (r *WorkflowRef) MetaPath(baseDir, sha string) string {
	return filepath.Join(baseDir, r.Host, r.Owner, r.Repo, sha, ".meta")
}

// FilePath resolves the workflow file path inside a cached repository.
// If Path is empty or points to a directory, it looks for workflow.yaml then ocw.yaml.
func (r *WorkflowRef) FilePath(cachePath string) (string, error) {
	target := cachePath
	if r.Path != "" {
		target = filepath.Join(cachePath, r.Path)
	}

	// Explicit file path
	if r.Path != "" && (strings.HasSuffix(r.Path, ".yaml") || strings.HasSuffix(r.Path, ".yml")) {
		if _, err := os.Stat(target); err != nil {
			return "", fmt.Errorf("workflow file not found: %s", target)
		}
		return target, nil
	}

	// Directory (or repo root) — look for defaults
	for _, name := range []string{"workflow.yaml", "ocw.yaml"} {
		p := filepath.Join(target, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("no workflow file found in %s (looked for workflow.yaml, ocw.yaml)", target)
}

// CacheMeta stores metadata about a cached workflow.
type CacheMeta struct {
	Ref       string    `json:"ref"`
	RefType   string    `json:"refType"`
	SHA       string    `json:"sha"`
	FetchedAt time.Time `json:"fetchedAt"`
	Source    string    `json:"source"` // "http" or "git"
}

// Fetcher downloads and caches remote workflow repositories from GitHub.
type Fetcher struct {
	CacheDir string
	HTTP     *http.Client
}

// NewFetcher creates a Fetcher using the standard XDG cache directory:
//   <UserCacheDir>/ocw/workflows
func NewFetcher() (*Fetcher, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("get user cache dir: %w", err)
	}
	return NewFetcherWithDir(filepath.Join(cacheDir, "ocw", "workflows")), nil
}

// NewFetcherWithDir creates a Fetcher with an explicit cache directory.
func NewFetcherWithDir(cacheDir string) *Fetcher {
	return &Fetcher{
		CacheDir: cacheDir,
		HTTP:     http.DefaultClient,
	}
}

// Fetch resolves a WorkflowRef to an absolute workflow file path,
// downloading and caching the repository as needed.
//
// Steps:
//  1. Resolve the ref to a long SHA (via git ls-remote for branches/tags, or directly for raw SHAs).
//  2. Check cache for that SHA.
//  3. If not cached, download the archive from GitHub or fall back to git shallow fetch.
//  4. Return the resolved workflow file path.
func (f *Fetcher) Fetch(ref *WorkflowRef) (string, error) {
	sha, err := f.resolveSHA(ref)
	if err != nil {
		return "", fmt.Errorf("resolve ref %q: %w", ref.Ref, err)
	}

	cachePath := ref.CachePath(f.CacheDir, sha)
	if _, err := os.Stat(cachePath); err == nil {
		// Cache directory exists — check if the workflow file is there
		workflowFile, err := ref.FilePath(cachePath)
		if err == nil {
			return workflowFile, nil // Cache hit
		}
		return "", fmt.Errorf("cached workflow missing file: %w", err)
	}

	// Cache miss: need to fetch
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	// Try HTTP first (faster for public repos)
	downloaded, err := f.downloadByHTTP(ref, sha)
	if err == nil {
		if err := extractTarGz(downloaded, cachePath); err != nil {
			os.RemoveAll(downloaded)
			return "", fmt.Errorf("extract archive: %w", err)
		}
		os.Remove(downloaded)
		if err := f.writeMeta(ref, sha, cachePath, "http"); err != nil {
			return "", err
		}
		return ref.FilePath(cachePath)
	}

	// Fallback to git (works for private repos)
	if err := f.downloadByGit(ref, sha, cachePath); err != nil {
		return "", fmt.Errorf("fetch workflow %s: http failed (%v), git fallback failed (%w)", ref.URL(), err, err)
	}
	if err := f.writeMeta(ref, sha, cachePath, "git"); err != nil {
		return "", err
	}

	return ref.FilePath(cachePath)
}

// resolveSHA resolves a ref (branch, tag, or raw SHA) to a long SHA.
// For raw SHAs, it returns the SHA as-is (after validating length).
// For branches and tags, it uses git ls-remote.
func (f *Fetcher) resolveSHA(ref *WorkflowRef) (string, error) {
	if ref.RefType == "sha" {
		if !looksLikeSHA(ref.Ref) {
			return "", fmt.Errorf("invalid SHA %q: expected at least 40 hex characters", ref.Ref)
		}
		return ref.Ref, nil
	}

	var remoteRef string
	switch ref.RefType {
	case "branch":
		remoteRef = "refs/heads/" + ref.Ref
	case "tag":
		remoteRef = "refs/tags/" + ref.Ref
	default:
		return "", fmt.Errorf("unknown ref type %q", ref.RefType)
	}

	cmd := exec.Command("git", "ls-remote", ref.URL(), remoteRef)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git ls-remote failed: %w\noutput: %s", err, out)
	}

	sha := strings.Fields(string(out))
	if len(sha) < 1 {
		return "", fmt.Errorf("ref %q not found in %s", ref.Ref, ref.URL())
	}
	return sha[0], nil
}

// downloadByHTTP downloads the tar.gz archive from the remote host for the given SHA.
// Returns the path to the downloaded temporary file.
func (f *Fetcher) downloadByHTTP(ref *WorkflowRef, sha string) (string, error) {
	archiveURL := fmt.Sprintf("https://%s/%s/%s/archive/%s.tar.gz",
		ref.Host, ref.Owner, ref.Repo, sha)

	resp, err := f.HTTP.Get(archiveURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read a bit of the body for error context, but limit it
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Create temp file in cache dir's parent so rename is cheap
	tmpDir := filepath.Join(f.CacheDir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(tmpDir, "archive-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// downloadByGit fetches a single commit via git and checks it out into dest.
// This is used as a fallback when HTTP downloads fail (e.g., for private repos).
func (f *Fetcher) downloadByGit(ref *WorkflowRef, sha, dest string) error {
	// Clean up any partial directory
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("clean partial cache: %w", err)
		}
	}

	// Create parent dir
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Create a temporary directory for the git operations
	tmpDir, err := os.MkdirTemp(filepath.Dir(dest), ".git-checkout-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a repo and fetch the specific SHA
	cmds := [][]string{
		{"git", "init", tmpDir},
		{"git", "-C", tmpDir, "remote", "add", "origin", ref.URL()},
		{"git", "-C", tmpDir, "fetch", "--depth", "1", "origin", sha},
		{"git", "-C", tmpDir, "checkout", "FETCH_HEAD"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git command %q failed: %w\noutput: %s", strings.Join(args, " "), err, out)
		}
	}

	// Move the contents (without .git) to the destination
	if err := os.RemoveAll(filepath.Join(tmpDir, ".git")); err != nil {
		return fmt.Errorf("remove .git directory: %w", err)
	}
	if err := os.Rename(tmpDir, dest); err != nil {
		return fmt.Errorf("move checkout to cache: %w", err)
	}

	return nil
}

// writeMeta writes a CacheMeta JSON file for a cached workflow.
func (f *Fetcher) writeMeta(ref *WorkflowRef, sha, cachePath, source string) error {
	meta := CacheMeta{
		Ref:       ref.Ref,
		RefType:   ref.RefType,
		SHA:       sha,
		FetchedAt: time.Now().UTC(),
		Source:    source,
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	metaPath := ref.MetaPath(f.CacheDir, sha)
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("write meta file: %w", err)
	}
	return nil
}

// extractTarGz extracts a tar.gz archive to dest, stripping the top-level directory
// that GitHub adds (e.g., "repo-abc1234/").
func extractTarGz(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer gz.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Strip the top-level directory (e.g., "repo-abc1234/")
		name := header.Name
		if idx := strings.Index(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == "" {
			continue // Skip the top-level directory entry
		}

		target := filepath.Join(dest, filepath.Clean("/"+name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Skip symlinks for security — they could escape the extraction dir
			continue
		default:
			// Skip other types
		}
	}

	return nil
}

// looksLikeSHA checks if a string looks like a Git SHA (40+ hex characters).
func looksLikeSHA(s string) bool {
	if len(s) < 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// refKey produces a stable string key for a WorkflowRef, used for error messages and logging.
func (r *WorkflowRef) refKey() string {
	return fmt.Sprintf("%s/%s/%s@%s", r.Host, r.Owner, r.Repo, r.Ref)
}