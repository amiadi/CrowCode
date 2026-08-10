# Administrator Guide

This guide is for operators, maintainers, and team admins who deploy and manage CrowCode.

## 1. Operational Goals

- Provide stable local-first AI usage.
- Keep data persistent and recoverable.
- Enforce safe tool and script execution.
- Keep model, RAG, and permissions configuration consistent across users.

## 2. Installation and Runtime

Typical options:
- Build from source with Go.
- Use packaged binaries and launch as `crowcode`.

Validate runtime:

```bash
crowcode -v
```

## 3. Core Configuration

Main config locations:
- `$HOME/.crowcode.json`
- `$XDG_CONFIG_HOME/crowcode/.crowcode.json`
- `./.crowcode.json` (project-local)

Recommended admin baseline:

```json
{
  "localOnly": true,
  "autoCompact": true,
  "rag": {
    "enabled": true,
    "topK": 5,
    "minScore": 0.2,
    "maxContextChars": 3200,
    "embeddings": {
      "provider": "ollama",
      "model": "nomic-embed-text",
      "dimensions": 768,
      "timeoutMs": 15000
    }
  },
  "python": {
    "enabled": true,
    "interpreter": "python3",
    "defaultTimeoutMs": 60000,
    "requirePermission": true,
    "tools": {}
  }
}
```

## 4. Local and Container Ollama

Set endpoint before running CrowCode:

```bash
export LOCAL_ENDPOINT=http://127.0.0.1:11434
```

Use `localOnly: true` when you want to prevent non-local provider fallback.

You can also use the setup wizard to apply and persist these settings:

```bash
crowcode setup
```

TUI path for operators:
1. Start `crowcode`
2. Press `Ctrl+K`
3. Select `Setup Local Connection Wizard`

For scripted rollouts on managed machines:

```bash
crowcode setup \
  --yes \
  --provider ollama \
  --endpoint http://127.0.0.1:11434 \
  --local-only \
  --prefer-ollama-embedder
```

Admin note:
- `localEndpoint` is written to config and used by local provider and RAG embedding paths.
- If probe fails, setup can still be applied, but validate connectivity before user rollout.

## 5. SQLite Operations and Backups

CrowCode stores runtime artifacts in SQLite under the configured data directory.

Admin practices:
- Perform scheduled file-level backups of the data directory.
- Keep backup retention windows aligned with audit requirements.
- Verify restore procedure in a staging workspace.

Example backup command:

```bash
cp -a .crowcode .crowcode.backup.$(date +%Y%m%d-%H%M%S)
```

## 6. RAG Governance

Connectors supported in `rag index`:
- text and text-file
- web
- sql
- object_storage
- directory and remote_share (recursive)

Safety controls:
- Use `--max-files` and `--max-bytes` for large folder/share ingestion.
- Restrict paths to approved mounts.
- Prefer service accounts with read-only access to network shares.

## 7. Embedding Lifecycle and Backfill

When changing embedding backend or model, re-embed existing chunks:

```bash
crowcode rag backfill-embeddings --limit 0
```

Operational guidance:
- Run in maintenance windows for large corpora.
- Monitor runtime and storage growth.
- Re-run search smoke tests after backfill.

## 8. Python Tool Governance

Use allowlisted script paths and permission gates.

Admin checks:
- Script path is explicit and version-controlled.
- Runtime args are constrained and documented.
- Timeout and environment are set per tool.
- Permission prompts are enabled unless there is a strong reason otherwise.

## 9. Access and Permissions Model

CrowCode supports per-execution permission prompts for tools.

Recommended policy:
- Keep permission prompts enabled for shell and python tools.
- Avoid broad auto-approval in shared machines.
- Use least privilege on filesystem and network shares.

## 10. MCP Server Operations

CrowCode supports three MCP provisioning modes:
- built-in bundled profiles
- user/project JSON manifests
- offline local binary drop-in (no manifests)

Built-in MCP profile names:
- `caveman`
- `coding`
- `reverse_engineering`

Manifest discovery folders:
- `<data.directory>/mcp/servers/builtin`
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

Binary discovery folders:
- `<data.directory>/mcp/bin`
- `$XDG_CONFIG_HOME/crowcode/mcp/bin`
- `$HOME/.crowcode/mcp/bin`
- `<workspace>/.crowcode/mcp/bin`

Compatibility folders also scanned for binaries:
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

Offline rollout example:

```bash
mkdir -p /opt/crowcode/mcp/bin
cp /repo/artifacts/mcp-reverse /opt/crowcode/mcp/bin/
chmod +x /opt/crowcode/mcp/bin/mcp-reverse
```

Operational notes:
- Auto-discovered binary names are normalized from file names.
- Manifest-defined servers override binary auto-discovered servers with the same name.
- Explicit `mcpServers` config entries override all auto-discovered entries.

Audit effective MCP configuration:

```bash
crowcode mcp list
```

Filter examples:

```bash
crowcode mcp list --source explicit
crowcode mcp list --source manifest,binary
crowcode mcp list --type stdio
```

This prints the effective source (`explicit`, `manifest`, `binary`) and origin path for each loaded MCP server.

## 11. Monitoring and Troubleshooting

Common checks:

```bash
go test ./...
go build -o /tmp/crowcode .
```

RAG health checks:

```bash
crowcode rag search --query "health check" --limit 5
crowcode rag benchmark --query "health check" --limit 5 --runs 5
```

If ingestion from remote share fails:
- Verify mount availability and permissions.
- Test with a smaller `--max-files` and `--max-bytes`.
- Confirm path format (`/mnt/...`, `\\server\\share\\...`, `file://`, `smb://`).

## 12. Upgrade Checklist

1. Pull latest code and review migration files.
2. Backup SQLite data directory.
3. Build and run tests.
4. Start CrowCode and verify migrations apply.
5. Run RAG smoke tests.
6. Rebuild embeddings if embedding settings changed.
7. Validate user workflows in one pilot repo before broad rollout.
