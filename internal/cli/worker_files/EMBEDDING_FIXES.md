# Embedding Fixes for Runtime Abstraction

## Problem

The Go CLI embeds worker Python files at compile time. Our new runtime abstraction code wasn't being embedded, causing:

1. **Missing dependencies**: `claude-agent-sdk` not in requirements.txt
2. **Missing files**: `runtimes/` directory and all runtime files not embedded
3. **Missing service**: `agent_executor_v2.py` not embedded
4. **Worker failures**: Team execution failing because imports couldn't resolve

## What Was Fixed

### 1. Updated `requirements.txt`

**File**: `/cli/internal/cli/requirements.txt`

Added:
```txt
# Claude Code SDK for claude_code runtime
claude-agent-sdk>=0.1.0
```

This ensures the Claude Code SDK is installed when the worker starts.

### 2. Updated `worker_embed.go`

**File**: `/cli/internal/cli/worker_embed.go`

Added embed directives for:
```go
// Embed runtimes package
//go:embed worker_files/runtimes/__init__.py
var runtimesInit string

//go:embed worker_files/runtimes/base.py
var runtimesBase string

//go:embed worker_files/runtimes/factory.py
var runtimesFactory string

//go:embed worker_files/runtimes/default_runtime.py
var runtimesDefaultRuntime string

//go:embed worker_files/runtimes/claude_code_runtime.py
var runtimesClaudeCodeRuntime string

// Embed V2 services (runtime-abstracted)
//go:embed worker_files/services/agent_executor_v2.py
var agentExecutorServiceV2 string
```

### 3. Updated `worker_start.go`

**File**: `/cli/internal/cli/worker_start.go`

Added file writing logic in `RunLocal()` function (after line 259):

```go
// Create runtimes directory and write files
runtimesDir := fmt.Sprintf("%s/runtimes", workerDir)
if err := os.MkdirAll(runtimesDir, 0755); err != nil {
    return fmt.Errorf("❌ failed to create runtimes directory: %w", err)
}

if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", runtimesDir), []byte(runtimesInit), 0644); err != nil {
    return fmt.Errorf("❌ failed to write runtimes __init__.py: %w", err)
}

if err := os.WriteFile(fmt.Sprintf("%s/base.py", runtimesDir), []byte(runtimesBase), 0644); err != nil {
    return fmt.Errorf("❌ failed to write runtimes/base.py: %w", err)
}

if err := os.WriteFile(fmt.Sprintf("%s/factory.py", runtimesDir), []byte(runtimesFactory), 0644); err != nil {
    return fmt.Errorf("❌ failed to write runtimes/factory.py: %w", err)
}

if err := os.WriteFile(fmt.Sprintf("%s/default_runtime.py", runtimesDir), []byte(runtimesDefaultRuntime), 0644); err != nil {
    return fmt.Errorf("❌ failed to write runtimes/default_runtime.py: %w", err)
}

if err := os.WriteFile(fmt.Sprintf("%s/claude_code_runtime.py", runtimesDir), []byte(runtimesClaudeCodeRuntime), 0644); err != nil {
    return fmt.Errorf("❌ failed to write runtimes/claude_code_runtime.py: %w", err)
}

// Write agent_executor_v2.py (new runtime-abstracted version)
if err := os.WriteFile(fmt.Sprintf("%s/agent_executor_v2.py", servicesDir), []byte(agentExecutorServiceV2), 0644); err != nil {
    return fmt.Errorf("❌ failed to write services/agent_executor_v2.py: %w", err)
}
```

Added success message:
```go
fmt.Println("✓ Runtime abstraction layer deployed")
```

## Files That Get Embedded Now

### Directory Structure After Deployment

```
~/.kubiya/workers/{queue-id}/
├── worker.py
├── requirements.txt
├── control_plane_client.py
├── workflows/
│   ├── __init__.py
│   ├── agent_execution.py
│   └── team_execution.py
├── activities/
│   ├── __init__.py
│   ├── agent_activities.py
│   └── team_activities.py
├── models/
│   ├── __init__.py
│   └── inputs.py
├── services/
│   ├── __init__.py
│   ├── agent_executor.py
│   ├── agent_executor_v2.py      ← NEW
│   ├── team_executor.py
│   ├── session_service.py
│   ├── cancellation_manager.py
│   └── toolset_factory.py
├── utils/
│   ├── __init__.py
│   ├── retry_utils.py
│   └── streaming_utils.py
└── runtimes/                      ← NEW
    ├── __init__.py
    ├── base.py
    ├── factory.py
    ├── default_runtime.py
    └── claude_code_runtime.py
```

## How to Rebuild and Deploy

### 1. Rebuild the CLI

```bash
cd /Users/shaked/projects/orchestrator/agent-control-plane/cli

# Rebuild Go binary with new embedded files
go build -o kubiya

# Or if you have a Makefile
make build
```

### 2. Test Locally

```bash
# Start worker locally (will use embedded files)
./kubiya worker start --queue-id=<your-queue-id> --type=local
```

You should now see:
```
✓ Worker code deployed from embedded binaries
✓ Workflows and activities configured
✓ Runtime abstraction layer deployed  ← NEW
```

### 3. Verify Dependencies

After the worker starts, check the deployed directory:

```bash
ls -la ~/.kubiya/workers/<queue-id>/runtimes/
# Should show:
# __init__.py
# base.py
# factory.py
# default_runtime.py
# claude_code_runtime.py

ls -la ~/.kubiya/workers/<queue-id>/services/
# Should show:
# agent_executor.py
# agent_executor_v2.py  ← NEW
```

Check installed packages:
```bash
~/.kubiya/workers/<queue-id>/venv/bin/pip list | grep claude
# Should show: claude-agent-sdk
```

## Previous Error Explanation

### The "Team execution workflow failed" Error

This was happening because:

1. **Missing imports**: The team execution might have tried importing from `runtimes` or other new modules
2. **Dependency issues**: `claude-agent-sdk` wasn't installed
3. **File not found**: Python couldn't find the runtime abstraction files

With these fixes, all embedded files are now properly written to disk before the worker starts.

## Verification Checklist

After rebuilding:

- [ ] `requirements.txt` includes `claude-agent-sdk`
- [ ] Go binary embeds all new files (check with `go build -v`)
- [ ] Worker deploys `runtimes/` directory
- [ ] Worker installs `claude-agent-sdk` during pip install
- [ ] Worker starts without import errors
- [ ] Team execution no longer fails immediately

## Testing the Fix

### Test 1: Worker Startup
```bash
./kubiya worker start --queue-id=<queue-id> --type=local
```

**Expected output**:
```
✓ Worker code deployed from embedded binaries
✓ Workflows and activities configured
✓ Runtime abstraction layer deployed
📦 DEPENDENCIES
   Installing Python dependencies (temporalio, etc.)... done
✓ All dependencies installed
```

### Test 2: Import Runtime Modules
```bash
cd ~/.kubiya/workers/<queue-id>
./venv/bin/python -c "from runtimes import RuntimeFactory; print(RuntimeFactory.get_supported_runtimes())"
```

**Expected output**:
```
[RuntimeType.DEFAULT, RuntimeType.CLAUDE_CODE]
```

### Test 3: Check Claude SDK
```bash
cd ~/.kubiya/workers/<queue-id>
./venv/bin/python -c "import claude_agent_sdk; print('Claude SDK version:', claude_agent_sdk.__version__)"
```

**Expected**: No import errors

## Next Steps

1. **Rebuild CLI**: `go build -o kubiya`
2. **Test locally**: `./kubiya worker start --queue-id=<id> --type=local`
3. **Verify deployment**: Check files in `~/.kubiya/workers/<queue-id>/`
4. **Test execution**: Try running an agent task
5. **Monitor logs**: Check for any import or runtime errors

## Docker Deployment

If using Docker mode (`--type=docker`), ensure the Dockerfile includes these files:

```dockerfile
# Copy all worker files
COPY cli/internal/cli/worker_files/ /app/worker_files/

# Install dependencies
RUN pip install -r /app/worker_files/requirements.txt
```

The Docker image will need to be rebuilt as well.

## Rollback Plan

If issues arise:

1. Revert changes to these files:
   - `cli/internal/cli/requirements.txt`
   - `cli/internal/cli/worker_embed.go`
   - `cli/internal/cli/worker_start.go`

2. Rebuild CLI:
   ```bash
   git checkout HEAD~1 cli/internal/cli/
   go build -o kubiya
   ```

3. Restart worker with old binary

---

**Status**: ✅ All embedding issues fixed
**Ready for**: Rebuild and test
**Impact**: Existing workers need CLI rebuild
