package fork

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "HTTPS with .git",
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "HTTPS without .git",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "SSH with .git",
			url:       "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "SSH without .git",
			url:       "git@github.com:owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "HTTPS with complex owner",
			url:       "https://github.com/my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
			wantErr:   false,
		},
		{
			name:      "SSH with underscores",
			url:       "git@github.com:user_name/repo_name.git",
			wantOwner: "user_name",
			wantRepo:  "repo_name",
			wantErr:   false,
		},
		{
			name:    "Invalid URL",
			url:     "not-a-github-url",
			wantErr: true,
		},
		{
			name:    "GitLab URL",
			url:     "https://gitlab.com/owner/repo",
			wantErr: true,
		},
		{
			name:    "Missing repo",
			url:     "https://github.com/owner",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseGitHubURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("ParseGitHubURL() owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("ParseGitHubURL() repo = %v, want %v", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestForkInfo(t *testing.T) {
	// Test ForkInfo struct defaults
	info := &ForkInfo{
		IsFork:        true,
		OriginURL:     "https://github.com/me/repo",
		OriginOwner:   "me",
		OriginRepo:    "repo",
		UpstreamURL:   "https://github.com/upstream/repo",
		UpstreamOwner: "upstream",
		UpstreamRepo:  "repo",
	}

	if !info.IsFork {
		t.Error("Expected IsFork to be true")
	}
	if info.OriginOwner != "me" {
		t.Errorf("Expected OriginOwner to be 'me', got %s", info.OriginOwner)
	}
	if info.UpstreamOwner != "upstream" {
		t.Errorf("Expected UpstreamOwner to be 'upstream', got %s", info.UpstreamOwner)
	}
}

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "fork-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir
}

func TestHasUpstreamRemote(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Initially no upstream
	if HasUpstreamRemote(tmpDir) {
		t.Error("expected no upstream remote initially")
	}

	// Add upstream remote
	cmd := exec.Command("git", "remote", "add", "upstream", "https://github.com/upstream/repo")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add upstream: %v", err)
	}

	// Now should have upstream
	if !HasUpstreamRemote(tmpDir) {
		t.Error("expected upstream remote after adding")
	}
}

func TestAddUpstreamRemote(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	upstreamURL := "https://github.com/upstream/repo"

	// Add upstream to repo without one
	if err := AddUpstreamRemote(tmpDir, upstreamURL); err != nil {
		t.Fatalf("AddUpstreamRemote() failed: %v", err)
	}

	// Verify it was added
	if !HasUpstreamRemote(tmpDir) {
		t.Error("upstream remote not added")
	}

	// Verify URL
	cmd := exec.Command("git", "remote", "get-url", "upstream")
	cmd.Dir = tmpDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get upstream url: %v", err)
	}
	got := string(output)
	if got != upstreamURL+"\n" {
		t.Errorf("upstream URL = %q, want %q", got, upstreamURL)
	}

	// Update existing upstream
	newURL := "https://github.com/other/repo"
	if err := AddUpstreamRemote(tmpDir, newURL); err != nil {
		t.Fatalf("AddUpstreamRemote() update failed: %v", err)
	}

	cmd = exec.Command("git", "remote", "get-url", "upstream")
	cmd.Dir = tmpDir
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("failed to get upstream url after update: %v", err)
	}
	got = string(output)
	if got != newURL+"\n" {
		t.Errorf("upstream URL after update = %q, want %q", got, newURL)
	}
}

func TestDetectFork_NoOrigin(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// DetectFork should fail without origin
	_, err := DetectFork(tmpDir)
	if err == nil {
		t.Error("expected error when no origin remote")
	}
}

func TestDetectFork_WithOrigin(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Add origin
	cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/myuser/myrepo")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}

	// DetectFork should succeed and detect non-fork
	info, err := DetectFork(tmpDir)
	if err != nil {
		t.Fatalf("DetectFork() failed: %v", err)
	}

	if info.OriginOwner != "myuser" {
		t.Errorf("OriginOwner = %q, want %q", info.OriginOwner, "myuser")
	}
	if info.OriginRepo != "myrepo" {
		t.Errorf("OriginRepo = %q, want %q", info.OriginRepo, "myrepo")
	}
}

func TestDetectFork_WithUpstream(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Add origin
	cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/myuser/myrepo")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}

	// Add upstream (simulating a fork)
	cmd = exec.Command("git", "remote", "add", "upstream", "https://github.com/original/repo")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add upstream: %v", err)
	}

	// DetectFork should detect fork
	info, err := DetectFork(tmpDir)
	if err != nil {
		t.Fatalf("DetectFork() failed: %v", err)
	}

	if !info.IsFork {
		t.Error("expected IsFork to be true with upstream remote")
	}
	if info.UpstreamOwner != "original" {
		t.Errorf("UpstreamOwner = %q, want %q", info.UpstreamOwner, "original")
	}
	if info.UpstreamRepo != "repo" {
		t.Errorf("UpstreamRepo = %q, want %q", info.UpstreamRepo, "repo")
	}
}

func TestGetRemoteURL(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// No remote should return error
	_, err := getRemoteURL(tmpDir, "origin")
	if err == nil {
		t.Error("expected error for non-existent remote")
	}

	// Add origin
	cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/test/repo")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}

	// Now should work
	url, err := getRemoteURL(tmpDir, "origin")
	if err != nil {
		t.Fatalf("getRemoteURL() failed: %v", err)
	}
	if url != "https://github.com/test/repo" {
		t.Errorf("url = %q, want %q", url, "https://github.com/test/repo")
	}
}

func TestDetectFork_InvalidPath(t *testing.T) {
	// Test with non-existent path
	_, err := DetectFork(filepath.Join(os.TempDir(), "nonexistent-fork-test"))
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}
