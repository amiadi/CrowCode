# User Guide

This guide helps users get productive quickly and use CrowCode to its full potential.

Platform support baseline:
- CrowCode tools and features are designed to work on both Linux and Windows.
- For path-based commands, use native local paths, Windows UNC paths, or `file://` URIs as documented.

## 1. Quickstart

Start interactive mode:

```bash
crowcode
```

Run one prompt non-interactively:

```bash
crowcode -p "Summarize this repository architecture"
```

## 2. Productive Daily Workflow

1. Open CrowCode in your project folder.
2. Ask for a task with clear goal and constraints.
3. Let the assistant inspect files and run targeted commands.
4. Review edits and test results.
5. Continue in the same session to keep context.

## 2.1 Setup Wizard for Local Endpoints

If you use Ollama, vLLM, or SGLang, run the setup wizard first:

```bash
crowcode setup
```

Non-interactive example:

```bash
crowcode setup --yes --provider vllm --endpoint http://127.0.0.1:8000
```

What it configures:
- local endpoint URL (`localEndpoint`)
- optional local-only mode
- local model bootstrap and defaults

TUI flow:
1. Start `crowcode`
2. Press `Ctrl+K`
3. Pick `Setup Local Connection Wizard`
4. Choose provider, endpoint, and toggles, then apply

Quick keyboard setup shortcuts:
- `Ctrl+K`: all commands (now available in both Chat and Workspace pages)
- `Ctrl+G`: model parameter setup commands
- `Ctrl+P`: prompt setup commands (system/skills/agent overrides)

Model parameter commands include:
- temperature
- top_k
- context window size (`num_ctx`)
- min_p
- repeat penalty

Prompt setup commands create workspace-local prompt overrides under:
- `.crowcode/prompts/system.md`
- `.crowcode/prompts/skills.md`
- `.crowcode/prompts/agents/<agent>.md`

Supported agent prompt override names:
- `coder`
- `task`
- `summarizer`
- `title`

## 2.2 Built-in MCP Servers and Auto-Discovery

CrowCode now supports built-in MCP server profiles and automatic MCP discovery.

Bundled built-in profiles:
- `caveman`
- `coding`
- `reverse_engineering`

How it works:
- On startup, CrowCode seeds built-in MCP manifest files into your data directory.
- It auto-discovers MCP manifests from global and workspace directories.
- It auto-discovers local executable MCP binaries for offline usage without JSON manifests.
- Discovered MCP servers are enabled automatically and available to the assistant.

Discovery directories:
- `<data.directory>/mcp/servers/builtin`
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

Binary discovery directories (manifest-free mode):
- `<data.directory>/mcp/bin`
- `$XDG_CONFIG_HOME/crowcode/mcp/bin`
- `$HOME/.crowcode/mcp/bin`
- `<workspace>/.crowcode/mcp/bin`

Compatibility folders also scanned for binaries:
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

User-downloaded MCP servers:
- Drop a JSON manifest into one of the discovery directories.
- CrowCode auto-loads it at next start without manual config edits.

Offline local binary mode:
- Drop a pre-downloaded MCP server binary into a binary discovery folder.
- Mark it executable.
- Restart CrowCode.
- The binary is auto-registered as a `stdio` MCP server.

Example offline binary drop-in:

```bash
mkdir -p ./.crowcode/mcp/bin
cp ./downloads/reverse-mcp ./.crowcode/mcp/bin/
chmod +x ./.crowcode/mcp/bin/reverse-mcp
crowcode
```

Auto-generated server names:
- Derived from file name and normalized.
- Example: `reverse-mcp` -> `reverse_mcp`

Example manifest format (single server):

```json
{
  "name": "my_custom_server",
  "type": "stdio",
  "command": "npx",
  "args": [
    "-y",
    "my-mcp-server-package"
  ]
}
```

Example manifest format (multiple named servers):

```json
{
  "my_server_a": {
    "type": "stdio",
    "command": "node",
    "args": ["./server-a.js"]
  },
  "my_server_b": {
    "type": "sse",
    "url": "http://127.0.0.1:8787/sse",
    "headers": {
      "Authorization": "Bearer token"
    }
  }
}
```

Variable expansion:
- `${CROWCODE_WORKDIR}` and `${OPENCODE_WORKDIR}` are expanded to your current workspace path inside `command`, `args`, and `env`.

Precedence:
- Explicit config `mcpServers` entries override all discovered servers.
- Manifest-discovered servers override binary-discovered servers with the same name.

Inspect what is active:

```bash
crowcode mcp list
```

Filter by source:

```bash
crowcode mcp list --source explicit
crowcode mcp list --source manifest,binary
```

Filter by transport type:

```bash
crowcode mcp list --type stdio
crowcode mcp list --type sse
```

It prints each effective server with source (`explicit`, `manifest`, `binary`), transport type (`stdio`, `sse`), endpoint, and origin path.

## 3. Prompting Patterns

Better prompts include:
- Objective: what you need done.
- Scope: files, package, or module.
- Constraints: style, performance, safety, compatibility.
- Validation: tests or build commands to run.

Example:

```text
Add recursive RAG folder ingestion for Linux and Windows shares.
Keep existing CLI behavior backward-compatible.
Add tests and update README examples.
Run go test ./... and report results.
```

## 4. RAG Ingestion

Supported offline extraction formats:
- plain text and markup: `txt`, `md`, `markdown`, `rst`, `json`, `yaml`, `yml`, `xml`, `html`, `htm`
- tabular: `csv`, `tsv`, `xlsx`
- documents: `docx`, `pdf`

This applies to `--text-file`, `object_storage` with `file://`, and directory/share ingestion.

### 4.1 File and text

```bash
crowcode rag index --uri local://notes --text-file ./notes.md --source-type local_file
```

Examples:

```bash
crowcode rag index --source-type local_file --uri local://manual --text-file ./manual.pdf
crowcode rag index --source-type local_file --uri local://spec --text-file ./spec.docx
crowcode rag index --source-type local_file --uri local://sheet --text-file ./registers.xlsx
```

### 4.2 Directory recursive ingestion

```bash
crowcode rag index \
  --source-type directory \
  --uri folder://docs \
  --directory-path ./docs \
  --recursive \
  --max-files 1000 \
  --max-bytes 33554432
```

### 4.3 Remote share ingestion

Linux mounted share:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://linux-team \
  --directory-path /mnt/team-share/knowledge \
  --recursive
```

Windows UNC path:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://windows-team \
  --directory-path "\\\\server\\share\\knowledge" \
  --recursive
```

SMB URI format:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://smb-team \
  --directory-path smb://server/share/knowledge \
  --recursive
```

### 4.4 Search and benchmark

```bash
crowcode rag search --query "local only ollama" --limit 5
crowcode rag benchmark --query "local only ollama" --limit 5 --runs 10 --expected-term ollama
```

### 4.5 Re-embed after model changes

```bash
crowcode rag backfill-embeddings --limit 0
```

## 5. Tool Usage Basics

CrowCode can use tools during tasks:
- file listing and reads
- search and grep
- patch/edit operations
- shell execution
- optional python tools

Best practice:
- Ask for small, testable changes.
- Ask the assistant to run tests after edits.
- Request a summary of changed files and behavior impact.

## 6. Python Tool Usage

If python tools are configured, they appear as `python.<tool_name>`.

Usage tips:
- Keep scripts deterministic and idempotent.
- Prefer JSON output for machine readability.
- Keep timeouts realistic.

## 7. Session and Compaction

CrowCode auto-compaction can summarize long sessions.

Use this to your advantage:
- Keep one mission per session.
- Ask the assistant to summarize and propose next action when a task segment ends.

## 8. Troubleshooting

If results are weak:
- Improve prompt clarity and constraints.
- Index better source material into RAG.
- Increase topK cautiously and keep minScore sensible.

If command fails:
- Re-run with explicit flags and exact paths.
- Verify folder/share access rights.
- Ask assistant to print a minimal reproducible command.

## 9. High-Value Habits

- Use project-local config for repo-specific defaults.
- Keep RAG sources focused on relevant docs and code.
- Reindex after major documentation changes.
- Ask for final verification steps before concluding tasks.
