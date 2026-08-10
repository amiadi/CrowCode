# Tools Reference

This reference explains common tool categories and how to use them effectively in CrowCode workflows.

## 1. File and Search Tools

Typical actions:
- list directories
- read files
- search text patterns
- find files by glob

Use when:
- mapping code structure
- locating symbols and configs
- verifying implementation points before edits

## 2. Edit and Patch Tools

Typical actions:
- apply focused code patches
- create new files
- update existing files

Use when:
- implementing requested changes
- adding tests
- updating documentation

Best practice:
- keep edits minimal and scoped
- avoid unrelated reformatting

## 3. Shell Command Tooling

Typical actions:
- run build/test commands
- run project CLIs
- execute smoke tests

Common commands:

```bash
go test ./...
go build -o /tmp/crowcode .
```

Use when:
- validating behavior after edits
- reproducing runtime issues

## 4. RAG Command Surface

Main CLI operations:

```bash
crowcode rag index ...
crowcode rag search ...
crowcode rag benchmark ...
crowcode rag backfill-embeddings ...
```

Directory and share connector flags:
- `--directory-path`
- `--recursive`
- `--max-files`
- `--max-bytes`

Supported path styles:
- Linux mounts (`/mnt/...`)
- Windows UNC (`\\server\\share\\...`)
- `file://...`
- `smb://...`

Supported offline extracted formats for `rag index` file ingestion:
- plain text and markup: `txt`, `md`, `markdown`, `rst`, `json`, `yaml`, `yml`, `xml`, `html`, `htm`
- tabular: `csv`, `tsv`, `xlsx`
- documents: `docx`, `pdf`

## 5. Connection Setup Wizard

Main setup command:

```bash
crowcode setup
```

Non-interactive setup example:

```bash
crowcode setup \
	--yes \
	--provider ollama \
	--endpoint http://127.0.0.1:11434 \
	--local-only \
	--prefer-ollama-embedder
```

Common flags:
- `--provider` (`ollama`, `vllm`, `sglang`)
- `--endpoint`
- `--local-only`
- `--prefer-ollama-embedder`
- `--no-probe`
- `--yes`

## 6. Python Tools

Python tools are registered through config and run as `python.<name>`.

Recommended script design:
- clear inputs and outputs
- deterministic behavior
- timeout awareness
- explicit failure messages

## 7. Permission and Safety Controls

CrowCode supports permission checks for sensitive actions.

Practical guidance:
- keep permission prompts enabled in multi-user contexts
- treat remote share reads as sensitive
- avoid broad auto-approval unless required

## 8. Troubleshooting Reference

When a tool fails:
1. check exact path and flags
2. run smallest reproducible command
3. validate environment variables
4. re-run tests after fixes

For RAG connector issues:
- confirm readable text content exists
- verify share mount and credentials
- lower ingestion limits to isolate failures

For setup wizard issues:
- verify endpoint reaches `/v1/models` or `/api/v0/models`
- check firewall/routing from the client host to the model server
- use `--no-probe` only when endpoint readiness is handled out of band

## 9. MCP Auto-Discovery

CrowCode auto-discovers MCP servers via:
- built-in profiles
- JSON manifests
- local binaries dropped into discovery folders

Built-in profile names:
- `caveman`
- `coding`
- `reverse_engineering`

Manifest folders:
- `<data.directory>/mcp/servers/builtin`
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

Binary folders (no JSON needed):
- `<data.directory>/mcp/bin`
- `$XDG_CONFIG_HOME/crowcode/mcp/bin`
- `$HOME/.crowcode/mcp/bin`
- `<workspace>/.crowcode/mcp/bin`

Quick binary drop-in example:

```bash
mkdir -p ./.crowcode/mcp/bin
cp ./downloads/my-mcp-server ./.crowcode/mcp/bin/
chmod +x ./.crowcode/mcp/bin/my-mcp-server
```

Precedence:
- Explicit config `mcpServers` > manifest discovery > binary discovery.

Inspection command:

```bash
crowcode mcp list
```

Filter by source:

```bash
crowcode mcp list --source explicit
crowcode mcp list --source manifest,binary
```

Filter by type:

```bash
crowcode mcp list --type stdio
crowcode mcp list --type sse
```

Output includes source classification (`explicit`, `manifest`, `binary`) and origin path for each effective MCP server.
