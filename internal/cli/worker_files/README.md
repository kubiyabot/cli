# Worker Files - Deprecated

⚠️ **This directory is deprecated and kept for reference only.**

## New Location

The worker code has been moved to a proper package at:
```
/agent-control-plane/worker/
```

## Why the Change?

The worker code was originally embedded directly in the CLI for distribution. However, this made it difficult to:
- Develop and test the worker independently
- Share the worker code between the CLI and Docker deployments
- Maintain proper version control and documentation

## Current Setup

The CLI now references the canonical worker package via Go embed directives:
- `cli/internal/cli/worker_embed.go` embeds files from `../../../worker/`
- All worker development should happen in the `/worker` directory
- The CLI automatically includes the latest worker code when built

## Migration

If you have local modifications to files in this directory, please:
1. Move them to the corresponding files in `/agent-control-plane/worker/`
2. Test using the worker package directly
3. The CLI will automatically pick up changes when rebuilt

## Removal

This directory can be safely deleted after confirming all references have been updated.
