# CLI Package Code Quality Analysis

**Date:** 2026-01-22
**Package:** `internal/cli`
**Coverage:** 28.2%
**Lines:** 5,052

## Executive Summary

The `internal/cli` package is a 5,052-line monolithic file that handles all CLI commands. While functional, it has grown too large and would benefit from modularization. This document outlines findings and recommendations.

## Key Findings

### 1. File Size and Structure

The `cli.go` file is too large at 5,052 lines with 67+ methods. The logical groupings are:

| Category | Approx Lines | Methods |
|----------|-------------|---------|
| Version utilities | 30 | `GetVersion`, `IsDevVersion` |
| CLI structure | 60 | `Command`, `CLI`, constructors |
| Command execution | 110 | `Execute`, `executeCommand`, help methods |
| Command registration | 340 | `registerCommands` |
| Daemon commands | 320 | `startDaemon`, `stopDaemon`, `daemonStatus`, `daemonLogs`, `stopAll` |
| Repository commands | 725 | `initRepo`, `listRepos`, `removeRepo`, config methods |
| Worker commands | 750 | `createWorker`, `listWorkers`, `showHistory`, `removeWorker` |
| Workspace commands | 490 | `addWorkspace`, `removeWorkspace`, `listWorkspaces`, `connectWorkspace` |
| Agent messaging | 140 | `sendMessage`, `listMessages`, `readMessage`, `ackMessage` |
| Context inference | 235 | `inferRepoFromCwd`, `resolveRepo`, `inferAgentContext` |
| Utilities | 15 | `formatTime`, `truncateString` |
| Agent management | 540 | `completeWorker`, `restartAgentCmd`, `reviewPR`, logs methods |
| Cleanup/repair | 640 | `cleanup`, `localCleanup`, `repair`, `localRepair` |
| Documentation | 60 | `showDocs`, `GenerateDocumentation` |
| Flag parsing | 40 | `ParseFlags` |
| Prompt utilities | 120 | `writePromptFile`, `writeMergeQueuePromptFile`, `writeWorkerPromptFile` |
| Claude startup | 80 | `startClaudeInTmux`, `setupOutputCapture` |

### 2. Dead Code

**`SelectFromListWithDefault`** in `selector.go` (lines 90-101) is defined but never used anywhere in the codebase.

```go
// This function is never called
func SelectFromListWithDefault(prompt string, items []SelectableItem, defaultValue string) (string, error) {
    selected, err := SelectFromList(prompt, items)
    if err != nil {
        return "", err
    }
    if selected == "" {
        return defaultValue, nil
    }
    return selected, nil
}
```

### 3. Code Duplication

#### A. Status Formatting (3+ occurrences)

The same switch statement for formatting status cells with colors appears in:
- `listWorkers` (lines 1996-2005)
- `showHistory` (lines 2178-2191)
- `listWorkspaces` (lines 2823-2833)

```go
// Repeated pattern
var statusCell format.ColoredCell
switch status {
case "running":
    statusCell = format.ColorCell(format.ColoredStatus(format.StatusRunning), nil)
case "completed":
    statusCell = format.ColorCell(format.ColoredStatus(format.StatusCompleted), nil)
case "stopped":
    statusCell = format.ColorCell(format.ColoredStatus(format.StatusError), nil)
default:
    statusCell = format.ColorCell(format.ColoredStatus(format.StatusIdle), nil)
}
```

**Recommendation:** Extract to `formatStatusCell(status string) format.ColoredCell`

#### B. Agent Creation Pattern

`createWorker`, `addWorkspace`, and parts of `initRepo` share nearly identical patterns:
1. Parse flags and validate arguments
2. Resolve repository
3. Create worktree
4. Create tmux window
5. Generate session ID
6. Write prompt file
7. Copy hooks configuration
8. Start Claude (if not in test mode)
9. Register with daemon

**Recommendation:** Extract common agent creation logic into a helper method.

#### C. Agent Removal Pattern

`removeWorker` and `removeWorkspace` follow nearly identical patterns:
1. Parse flags
2. Resolve repository
3. Get agent info from daemon
4. Check for uncommitted changes
5. Prompt for confirmation
6. Kill tmux window
7. Remove worktree
8. Unregister from daemon

**Recommendation:** Extract common agent removal logic.

#### D. Daemon Client Pattern

The pattern `client := socket.NewClient(c.paths.DaemonSock)` followed by error handling is repeated extensively.

**Recommendation:** Add helper method `(c *CLI) daemonClient() *socket.Client`

### 4. Test Coverage Gaps

Current coverage: **28.2%**

**Methods with no integration tests:**
- `initRepo` - Only name parsing tested, not full flow
- `showHistory` - No tests
- `reviewPR` - Only invalid URL case tested
- `restartClaude` - No tests
- `cleanupMergedBranches` - No tests
- `viewLogs`, `searchLogs`, `cleanLogs` - Limited coverage

**Methods with good coverage:**
- `ParseFlags`
- `formatTime`, `truncateString`
- `GenerateDocumentation`
- Agent messaging (`sendMessage`, `listMessages`, etc.)
- Socket communication

### 5. Complexity Hotspots

**`initRepo` (lines 943-1273)** - 330 lines, does too much:
- Validates input
- Clones repository
- Creates tmux session
- Creates multiple agents (supervisor, merge-queue, workspace)
- Each with prompt files, hooks, Claude startup

**`localCleanup` (lines 4177-4447)** - 270 lines of nested loops and conditionals

**`stopAll` (lines 718-941)** - 223 lines with `--clean` flag adding significant complexity

## Recommendations

### Phase 1: Quick Wins (Low Risk)

1. **Remove dead code**: Delete `SelectFromListWithDefault` from `selector.go`

2. **Extract status formatting helper**:
   ```go
   func formatAgentStatusCell(status string) format.ColoredCell
   ```

3. **Add daemon client helper**:
   ```go
   func (c *CLI) daemonClient() *socket.Client
   ```

### Phase 2: File Splitting (Medium Risk)

Split `cli.go` into logical files while keeping them in the same package:

| New File | Contents |
|----------|----------|
| `cli_daemon.go` | Daemon commands (`startDaemon`, `stopDaemon`, etc.) |
| `cli_repo.go` | Repository commands (`initRepo`, `listRepos`, etc.) |
| `cli_worker.go` | Worker commands (`createWorker`, `listWorkers`, etc.) |
| `cli_workspace.go` | Workspace commands |
| `cli_agent.go` | Agent messaging commands |
| `cli_logs.go` | Log viewing commands |
| `cli_maintenance.go` | Cleanup and repair commands |
| `cli_util.go` | Utility functions and helpers |

This is purely organizational and preserves all behavior.

### Phase 3: Test Coverage Improvement (Medium Effort)

Priority tests to add:
1. `initRepo` full integration test
2. `showHistory` with various filters
3. `cleanupMergedBranches`
4. Log commands (`viewLogs`, `searchLogs`)

### Phase 4: Refactoring (Higher Risk)

1. Extract agent creation helper to reduce duplication
2. Extract agent removal helper
3. Break down `initRepo` into smaller functions

## Metrics to Track

- Coverage: Target 50%+ (currently 28.2%)
- Largest file: Target <1000 lines (currently 5,052)
- Max function length: Target <100 lines

## Action Items

- [ ] Delete dead code (`SelectFromListWithDefault`)
- [ ] Extract `formatAgentStatusCell` helper
- [ ] Split `cli.go` into logical files
- [ ] Add tests for `initRepo`, `showHistory`
- [ ] Extract common agent creation/removal patterns
