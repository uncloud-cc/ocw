package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/uncloud-cc/ocw/pkg/schema"
)

// FileWatcher watches for file system changes and triggers callbacks
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	watchConfig *schema.Watch
	basePath    string
	ignoreFns   []func(string) bool
	onChange    func(changedFile string)
	debounce    time.Duration

	mu              sync.Mutex
	timer           *time.Timer
	lastChangedFile string
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(watchConfig *schema.Watch, basePath string, onChange func(changedFile string)) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWatcher{
		watcher:     watcher,
		watchConfig: watchConfig,
		basePath:    basePath,
		onChange:    onChange,
		debounce:    100 * time.Millisecond,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Build ignore functions from .gitignore, .dockerignore, and custom patterns
	fw.buildIgnoreFns()

	// Set up watches on directories
	if err := fw.setupWatches(); err != nil {
		watcher.Close()
		cancel()
		return nil, err
	}

	return fw, nil
}

// buildIgnoreFns builds the list of ignore check functions
func (fw *FileWatcher) buildIgnoreFns() {
	// Load .gitignore patterns
	if fw.watchConfig.ShouldUseGitIgnore() {
		if patterns := fw.loadIgnoreFile(".gitignore"); len(patterns) > 0 {
			fw.ignoreFns = append(fw.ignoreFns, fw.makeIgnoreFn(patterns))
		}
	}

	// Load .dockerignore patterns
	if fw.watchConfig.ShouldUseDockerIgnore() {
		if patterns := fw.loadIgnoreFile(".dockerignore"); len(patterns) > 0 {
			fw.ignoreFns = append(fw.ignoreFns, fw.makeIgnoreFn(patterns))
		}
	}

	// Add custom ignore patterns
	if customPatterns := fw.watchConfig.GetIgnorePatterns(); len(customPatterns) > 0 {
		fw.ignoreFns = append(fw.ignoreFns, fw.makeIgnoreFn(customPatterns))
	}

	// Always ignore .git directory
	fw.ignoreFns = append(fw.ignoreFns, func(path string) bool {
		relPath, _ := filepath.Rel(fw.basePath, path)
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if part == ".git" {
				return true
			}
		}
		return false
	})
}

// loadIgnoreFile loads patterns from a .gitignore or .dockerignore file
func (fw *FileWatcher) loadIgnoreFile(filename string) []string {
	path := filepath.Join(fw.basePath, filename)
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// makeIgnoreFn creates an ignore function from patterns
func (fw *FileWatcher) makeIgnoreFn(patterns []string) func(string) bool {
	return func(path string) bool {
		relPath, err := filepath.Rel(fw.basePath, path)
		if err != nil {
			return false
		}

		for _, pattern := range patterns {
			// Handle negation (not fully supported, just skip)
			if strings.HasPrefix(pattern, "!") {
				continue
			}

			// Handle directory-only patterns
			isDir := strings.HasSuffix(pattern, "/")
			if isDir {
				pattern = strings.TrimSuffix(pattern, "/")
			}

			// Try matching the pattern against the full relative path
			if matched, _ := filepath.Match(pattern, relPath); matched {
				return true
			}

			// Try matching against the basename
			if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
				return true
			}

			// Try matching pattern against each path component
			parts := strings.Split(relPath, string(filepath.Separator))
			for i := range parts {
				subPath := filepath.Join(parts[:i+1]...)
				if matched, _ := filepath.Match(pattern, subPath); matched {
					return true
				}
				if matched, _ := filepath.Match(pattern, parts[i]); matched {
					return true
				}
			}

			// Handle ** patterns (double-star glob)
			if strings.Contains(pattern, "**") {
				// Convert ** to a regex-like match
				regexPattern := strings.ReplaceAll(pattern, "**", ".*")
				regexPattern = strings.ReplaceAll(regexPattern, "*", "[^/]*")
				// Simple substring match for ** patterns
				if strings.Contains(relPath, strings.ReplaceAll(strings.ReplaceAll(pattern, "**", ""), "*", "")) {
					return true
				}
			}
		}
		return false
	}
}

// setupWatches sets up file system watches based on config
func (fw *FileWatcher) setupWatches() error {
	patterns := fw.watchConfig.GetFiles()

	// If no specific patterns, watch entire base path
	if len(patterns) == 0 {
		return fw.watchRecursive(fw.basePath)
	}

	// Watch specific patterns
	for _, pattern := range patterns {
		// Make pattern absolute if it's relative
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(fw.basePath, pattern)
		}

		// Get the directory part of the pattern (before any glob characters)
		dirToWatch := fw.getWatchDir(pattern)

		if err := fw.watchRecursive(dirToWatch); err != nil {
			return err
		}
	}

	return nil
}

// getWatchDir extracts the directory to watch from a glob pattern
func (fw *FileWatcher) getWatchDir(pattern string) string {
	// Find the first path component with glob characters
	// Everything before that is the directory to watch

	// Replace glob characters with separator to find the cut point
	idx := strings.IndexAny(pattern, "*?[")

	var dir string
	if idx == -1 {
		// No glob characters, use the whole pattern
		dir = pattern
	} else {
		// Find the last separator before the first glob character
		beforeGlob := pattern[:idx]
		lastSep := strings.LastIndex(beforeGlob, string(filepath.Separator))
		if lastSep == -1 {
			// No separator before glob, use base path
			dir = fw.basePath
		} else {
			dir = pattern[:lastSep]
		}
	}

	// If the path is a file, get its directory
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	return dir
}

// watchRecursive recursively adds watches to a directory
func (fw *FileWatcher) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check if should ignore
		if fw.shouldIgnore(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only watch directories (fsnotify watches files in watched dirs)
		if info.IsDir() {
			if err := fw.watcher.Add(path); err != nil {
				// Don't fail on permission errors
				if !os.IsPermission(err) {
					return err
				}
			}
		}

		return nil
	})
}

// shouldIgnore checks if a path should be ignored
func (fw *FileWatcher) shouldIgnore(path string) bool {
	for _, fn := range fw.ignoreFns {
		if fn(path) {
			return true
		}
	}
	return false
}

// shouldTriggerOnFile checks if a file change should trigger a reload
func (fw *FileWatcher) shouldTriggerOnFile(path string) bool {
	// First check ignore patterns
	if fw.shouldIgnore(path) {
		return false
	}

	// If specific file patterns are set, check them
	patterns := fw.watchConfig.GetFiles()
	if len(patterns) == 0 {
		return true // No specific patterns, trigger on any non-ignored file
	}

	relPath, err := filepath.Rel(fw.basePath, path)
	if err != nil {
		return false
	}

	// Normalize path separators for glob matching (doublestar expects forward slashes)
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		// Make pattern relative if needed
		if filepath.IsAbs(pattern) {
			pattern, _ = filepath.Rel(fw.basePath, pattern)
		}

		// Normalize pattern separators
		pattern = filepath.ToSlash(pattern)

		// Remove leading "./" from pattern if present (for consistency)
		if strings.HasPrefix(pattern, "./") {
			pattern = pattern[2:]
		}

		// Use doublestar for proper glob matching
		matched, err := doublestar.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// Start begins watching for file changes
func (fw *FileWatcher) Start() {
	go fw.watchLoop()
}

// watchLoop is the main event loop
func (fw *FileWatcher) watchLoop() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// Log but don't stop
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)

		case <-fw.ctx.Done():
			return
		}
	}
}

// handleEvent processes a file system event
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// Only care about write, create, remove, rename
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) &&
		!event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
		return
	}

	// Check if this file should trigger a reload
	if !fw.shouldTriggerOnFile(event.Name) {
		return
	}

	// Handle new directories (add watches)
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			fw.watchRecursive(event.Name)
		}
	}

	// Trigger change with debouncing, tracking the changed file
	fw.triggerChange(event.Name)
}

// triggerChange triggers the onChange callback with debouncing
func (fw *FileWatcher) triggerChange(changedFile string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Store the changed file path
	fw.lastChangedFile = changedFile

	// Cancel any pending timer
	if fw.timer != nil {
		fw.timer.Stop()
	}

	// Start new debounce timer
	fw.timer = time.AfterFunc(fw.debounce, func() {
		fw.mu.Lock()
		changedFile := fw.lastChangedFile
		fw.mu.Unlock()
		fw.onChange(changedFile)
	})
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	fw.cancel()
	return fw.watcher.Close()
}
