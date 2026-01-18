package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dlorenc/multiclaude/internal/messages"
	"github.com/dlorenc/multiclaude/internal/socket"
	"github.com/dlorenc/multiclaude/internal/state"
	"github.com/dlorenc/multiclaude/pkg/config"
)

func setupTestDaemon(t *testing.T) (*Daemon, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "daemon-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create paths
	paths := &config.Paths{
		Root:         tmpDir,
		DaemonPID:    filepath.Join(tmpDir, "daemon.pid"),
		DaemonSock:   filepath.Join(tmpDir, "daemon.sock"),
		DaemonLog:    filepath.Join(tmpDir, "daemon.log"),
		StateFile:    filepath.Join(tmpDir, "state.json"),
		ReposDir:     filepath.Join(tmpDir, "repos"),
		WorktreesDir: filepath.Join(tmpDir, "wts"),
		MessagesDir:  filepath.Join(tmpDir, "messages"),
	}

	// Create directories
	if err := paths.EnsureDirectories(); err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	// Create daemon
	d, err := New(paths)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return d, cleanup
}

func TestDaemonCreation(t *testing.T) {
	d, cleanup := setupTestDaemon(t)
	defer cleanup()

	if d == nil {
		t.Fatal("Daemon should not be nil")
	}

	if d.state == nil {
		t.Fatal("Daemon state should not be nil")
	}

	if d.tmux == nil {
		t.Fatal("Daemon tmux client should not be nil")
	}

	if d.logger == nil {
		t.Fatal("Daemon logger should not be nil")
	}
}

func TestGetMessageManager(t *testing.T) {
	d, cleanup := setupTestDaemon(t)
	defer cleanup()

	mgr := d.getMessageManager()
	if mgr == nil {
		t.Fatal("Message manager should not be nil")
	}
}

func TestRouteMessages(t *testing.T) {
	d, cleanup := setupTestDaemon(t)
	defer cleanup()

	// Add a test repository
	repo := &state.Repository{
		GithubURL:   "https://github.com/test/repo",
		TmuxSession: "test-session",
		Agents:      make(map[string]state.Agent),
	}
	if err := d.state.AddRepo("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repo: %v", err)
	}

	// Add a test agent
	agent := state.Agent{
		Type:         state.AgentTypeWorker,
		WorktreePath: "/tmp/test",
		TmuxWindow:   "test-window",
		SessionID:    "test-session-id",
		CreatedAt:    time.Now(),
	}
	if err := d.state.AddAgent("test-repo", "test-agent", agent); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Create a message
	msgMgr := messages.NewManager(d.paths.MessagesDir)
	msg, err := msgMgr.Send("test-repo", "supervisor", "test-agent", "Test message body")
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Verify message is pending
	if msg.Status != messages.StatusPending {
		t.Errorf("Message status = %s, want %s", msg.Status, messages.StatusPending)
	}

	// Call routeMessages (it will try to send via tmux, which will fail, but that's ok)
	d.routeMessages()

	// Note: We can't verify delivery without a real tmux session,
	// but we've tested that the function doesn't panic
}

func TestCleanupDeadAgents(t *testing.T) {
	d, cleanup := setupTestDaemon(t)
	defer cleanup()

	// Add a test repository
	repo := &state.Repository{
		GithubURL:   "https://github.com/test/repo",
		TmuxSession: "test-session",
		Agents:      make(map[string]state.Agent),
	}
	if err := d.state.AddRepo("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repo: %v", err)
	}

	// Add a test agent
	agent := state.Agent{
		Type:         state.AgentTypeWorker,
		WorktreePath: "/tmp/test",
		TmuxWindow:   "test-window",
		SessionID:    "test-session-id",
		CreatedAt:    time.Now(),
	}
	if err := d.state.AddAgent("test-repo", "test-agent", agent); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Verify agent exists
	_, exists := d.state.GetAgent("test-repo", "test-agent")
	if !exists {
		t.Fatal("Agent should exist before cleanup")
	}

	// Mark agent as dead
	deadAgents := map[string][]string{
		"test-repo": {"test-agent"},
	}

	// Call cleanup
	d.cleanupDeadAgents(deadAgents)

	// Verify agent was removed
	_, exists = d.state.GetAgent("test-repo", "test-agent")
	if exists {
		t.Error("Agent should not exist after cleanup")
	}
}

func TestHandleCompleteAgent(t *testing.T) {
	d, cleanup := setupTestDaemon(t)
	defer cleanup()

	// Add a test repository
	repo := &state.Repository{
		GithubURL:   "https://github.com/test/repo",
		TmuxSession: "test-session",
		Agents:      make(map[string]state.Agent),
	}
	if err := d.state.AddRepo("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repo: %v", err)
	}

	// Add a test agent
	agent := state.Agent{
		Type:         state.AgentTypeWorker,
		WorktreePath: "/tmp/test",
		TmuxWindow:   "test-window",
		SessionID:    "test-session-id",
		CreatedAt:    time.Now(),
	}
	if err := d.state.AddAgent("test-repo", "test-agent", agent); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Test missing repo argument
	resp := d.handleCompleteAgent(socket.Request{
		Command: "complete_agent",
		Args: map[string]interface{}{
			"agent": "test-agent",
		},
	})
	if resp.Success {
		t.Error("Expected failure with missing repo")
	}

	// Test missing agent argument
	resp = d.handleCompleteAgent(socket.Request{
		Command: "complete_agent",
		Args: map[string]interface{}{
			"repo": "test-repo",
		},
	})
	if resp.Success {
		t.Error("Expected failure with missing agent")
	}

	// Test non-existent agent
	resp = d.handleCompleteAgent(socket.Request{
		Command: "complete_agent",
		Args: map[string]interface{}{
			"repo":  "test-repo",
			"agent": "non-existent",
		},
	})
	if resp.Success {
		t.Error("Expected failure with non-existent agent")
	}

	// Test successful completion
	resp = d.handleCompleteAgent(socket.Request{
		Command: "complete_agent",
		Args: map[string]interface{}{
			"repo":  "test-repo",
			"agent": "test-agent",
		},
	})
	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}

	// Verify agent is marked for cleanup
	updatedAgent, _ := d.state.GetAgent("test-repo", "test-agent")
	if !updatedAgent.ReadyForCleanup {
		t.Error("Agent should be marked as ready for cleanup")
	}
}

func TestIsProcessAlive(t *testing.T) {
	// Test with PID 1 (init, should be alive on Unix systems)
	// This is more reliable than testing our own process
	if isProcessAlive(1) {
		t.Log("PID 1 is alive (as expected)")
	} else {
		t.Skip("PID 1 not available on this system")
	}

	// Test with very high invalid PID (should be dead)
	if isProcessAlive(999999) {
		t.Error("Invalid PID 999999 should be reported as dead")
	}
}
