package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uncloud-cc/ocw/pkg/schema"
)

func TestFileWatcher_BasicWatch(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create watch config
	boolTrue := true
	watch := &schema.Watch{Bool: &boolTrue}

	// Track changes
	changes := make(chan struct{}, 10)
	onChange := func() {
		changes <- struct{}{}
	}

	// Create watcher
	watcher, err := NewFileWatcher(watch, tmpDir, func(changedFile string) {
		onChange()
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	watcher.Start()

	// Give watcher time to set up
	time.Sleep(100 * time.Millisecond)

	// Modify file
	if err := os.WriteFile(testFile, []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for change
	select {
	case <-changes:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for file change")
	}
}

func TestFileWatcher_IgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore
	gitignore := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("*.log\nnode_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	boolTrue := true
	watch := &schema.Watch{Bool: &boolTrue}

	changes := make(chan struct{}, 10)
	watcher, err := NewFileWatcher(watch, tmpDir, func(changedFile string) {
		changes <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()

	// Test that .log files are ignored
	if !watcher.shouldIgnore(filepath.Join(tmpDir, "test.log")) {
		t.Error("Expected .log files to be ignored")
	}

	// Test that regular files are not ignored
	if watcher.shouldIgnore(filepath.Join(tmpDir, "test.txt")) {
		t.Error("Expected .txt files to not be ignored")
	}
}

func TestWatch_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		watch    *schema.Watch
		expected bool
	}{
		{"nil", nil, false},
		{"bool true", &schema.Watch{Bool: func() *bool { b := true; return &b }()}, true},
		{"bool false", &schema.Watch{Bool: func() *bool { b := false; return &b }()}, false},
		{"string pattern", &schema.Watch{String: func() *string { s := "*.go"; return &s }()}, true},
		{"string slice", &schema.Watch{Strings: []string{"*.go", "*.js"}}, true},
		{"config", &schema.Watch{Config: &schema.WatchConfig{Files: []string{"src/"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.watch.IsEnabled()
			if result != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}
