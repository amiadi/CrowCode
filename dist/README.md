# CrowCode: Linux and Windows x86_64 binary guide

## 1. What is included

| File | Purpose |
| --- | --- |
| `crowcode-linux-amd64` | Linux x86_64 executable |
| `crowcode-windows-amd64.exe` | Windows x86_64 executable |
| `SHA256SUMS` | SHA-256 checksums of these two executables |
| `LICENSE.txt` | CrowCode license |
| `THIRD_PARTY_NOTICES.txt` | Bundled tool licenses, versions, sources, and upstream archive checksums |
| `README.md` | This guide |

**Each executable contains ripgrep 15.2.0 (`rg`) and fzf 0.74.5.** Copying the
appropriate CrowCode executable is sufficient to run both tools: no separate
installation, internet access, Go, Rust, or package manager is needed.

CrowCode extracts only its platform's tools into a new private
`crowcode-tools-*` temporary directory on each launch. It uses those executables
for file listing, fuzzy file completion, and content searches, and prepends
their directory to its own subprocess `PATH`. It removes the directory after
normal completion, including command errors. Your system's `PATH` and installed
`rg`/`fzf` are not changed. Shell startup scripts can override inherited `PATH`;
use the shell configuration in section 4 to avoid this.

You can also run the embedded tools directly with `crowcode rg ...` and
`crowcode fzf ...`. Their native flags, standard input/output, and normal exit
codes are preserved. Shell completions and fzf shell key bindings are **not**
installed. `crowcode licenses` prints the embedded license notices.
Interrupt/termination signals during a direct tool command cancel and wait for
the subprocess, clean up the extracted tools, and exit with code `130`.

## 2. Requirements and verification

- An x86_64/AMD64 operating system, not ARM64 or 32-bit Windows.
- A terminal supporting ANSI escape sequences for the interactive UI
  (Windows Terminal is recommended on Windows).
- A writable temporary directory where executable files are permitted. Linux
  uses `TMPDIR` when set, otherwise `/tmp`; Windows uses Go's temporary directory
  selection, normally `%TMP%` or `%TEMP%`.
- A writable project/data directory for configuration, sessions, and SQLite.
- An AI provider: a running local model server or a configured cloud provider.
  Tool-only commands (`rg`, `fzf`, `licenses`, help, version) need neither.
- Bash for the assistant's shell tool; on Windows install Git for Windows
  separately and configure its Bash path. PowerShell can launch CrowCode, but
  the assistant's persistent shell currently uses POSIX shell syntax, not
  PowerShell/CMD syntax. Git, Python, LSP servers, MCP servers, and model servers
  are optional external components and are **not** bundled.

Obtain binaries and checksums from a trusted copy of this repository. Checksums
detect corruption, not authenticity if both files came from an untrusted source.
Run these commands from the downloaded `dist` folder.

Linux:

```bash
sha256sum -c SHA256SUMS --ignore-missing
chmod +x ./crowcode-linux-amd64
./crowcode-linux-amd64 --version
./crowcode-linux-amd64 rg --version
./crowcode-linux-amd64 fzf --version
```

Windows PowerShell:

```powershell
Get-Content .\SHA256SUMS | ForEach-Object {
    $expected, $name = $_ -split '\s+', 2
    $file = Join-Path $PWD $name
    if (Test-Path $file) {
        $actual = (Get-FileHash $file -Algorithm SHA256).Hash
        if ($actual -ne $expected) { throw "Checksum mismatch: $name" }
        Write-Host "OK: $name"
    }
}
.\crowcode-windows-amd64.exe --version
.\crowcode-windows-amd64.exe rg --version
.\crowcode-windows-amd64.exe fzf --version
```

## 3. Installation

### Linux: portable or per-user

For portable use, keep the executable anywhere and invoke its absolute path:

```bash
/path/to/dist/crowcode-linux-amd64 -c /path/to/project
```

For a per-user installation, from `dist`:

```bash
mkdir -p "$HOME/.local/bin" "$HOME/.local/share/crowcode"
install -m 0755 ./crowcode-linux-amd64 "$HOME/.local/bin/crowcode"
cp README.md LICENSE.txt THIRD_PARTY_NOTICES.txt "$HOME/.local/share/crowcode/"
export PATH="$HOME/.local/bin:$PATH"
crowcode --version
```

Add the `export PATH=...` line to your shell profile if `~/.local/bin` is not
already on your path. No root access is needed. For a shared installation an
administrator can instead install the executable in `/usr/local/bin/crowcode`
and the documentation/licenses in `/usr/local/share/crowcode`.

### Windows: portable or per-user

Portable use from PowerShell:

```powershell
& 'C:\Tools\CrowCode\crowcode-windows-amd64.exe' -c 'C:\Projects\MyProject'
```

For a per-user installation, from `dist`:

```powershell
$installDir = Join-Path $env:LOCALAPPDATA 'CrowCode\bin'
New-Item -ItemType Directory -Force $installDir | Out-Null
Copy-Item .\crowcode-windows-amd64.exe (Join-Path $installDir 'crowcode.exe')
Copy-Item .\README.md, .\LICENSE.txt, .\THIRD_PARTY_NOTICES.txt $installDir
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
}
$env:Path = "$installDir;$env:Path"
crowcode --version
```

New terminals pick up the persistent user `Path`. These binaries are not
code-signed. If Windows marks a trusted downloaded executable as blocked,
verify its checksum and source before using `Unblock-File` on that executable;
do not disable antivirus or execution controls globally.

## 4. Project configuration and models

Run CrowCode **in the project you want it to work on**, or use root `-c`.
Global JSON configuration is searched in this order:

1. `$HOME/.crowcode.json`
2. `$XDG_CONFIG_HOME/crowcode/.crowcode.json`
3. `$HOME/.config/crowcode/.crowcode.json`

The project `.crowcode.json` is merged over the selected global configuration.
The current loader uses `HOME`/XDG paths, not an automatic Windows AppData config
path. On Windows set `$env:HOME = $env:USERPROFILE` if `HOME` is absent; use that
in each launch environment, or explicitly configure `XDG_CONFIG_HOME`.

By default, data is in `<project>/.crowcode/`, including `opencode.db`, sessions,
RAG content, prompts, and MCP discovery files. `data.directory` can override it
with an absolute writable path. Keep `.crowcode/` and secret-bearing config out
of version control.

### Local Ollama (Linux and Windows)

Install and start Ollama separately. Download a coding model, and optionally
the RAG embedding model, before going offline:

```text
ollama pull qwen2.5-coder:7b
ollama pull nomic-embed-text
```

Set the endpoint and launch from your project:

Linux:

```bash
export LOCAL_ENDPOINT=http://127.0.0.1:11434
crowcode
```

Windows PowerShell:

```powershell
if (-not $env:HOME) { $env:HOME = $env:USERPROFILE }
$env:LOCAL_ENDPOINT = 'http://127.0.0.1:11434'
crowcode
```

Use `Ctrl+O` to select a discovered model, or `Ctrl+K` → **Setup Local Connection
Wizard** for saved endpoint settings. For persistent local-only settings,
merge these keys into the project `.crowcode.json` rather than overwriting
existing settings:

```json
{
  "localEndpoint": "http://127.0.0.1:11434",
  "localOnly": true,
  "shell": {
    "path": "/bin/bash",
    "args": ["--noprofile", "--norc"]
  }
}
```

On Windows replace the shell path with your actual Git Bash executable, e.g.
`"C:/Program Files/Git/bin/bash.exe"`. The non-login arguments preserve the
inherited bundled-tool `PATH` instead of loading profile scripts that reset it.
For vLLM or SGLang use your server's OpenAI-compatible endpoint (commonly
`http://127.0.0.1:8000` and `http://127.0.0.1:30000`).

`localOnly` limits model provider selection; it is **not** a network sandbox
for shell, web, or MCP tools. Keep services bound to loopback unless remote
access is intentionally secured. Containers must expose the model API to the
CrowCode host; `127.0.0.1` inside a container refers to that container.

### Cloud providers

Provide credentials through the process environment, not committed JSON.
Supported variables include `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`,
`GEMINI_API_KEY`, `GROQ_API_KEY`, `OPENROUTER_API_KEY`, `XAI_API_KEY`,
and Azure's `AZURE_OPENAI_ENDPOINT`/`AZURE_OPENAI_API_KEY`.

Example without putting an OpenAI key into shell history:

```bash
read -r -s -p "OpenAI API key: " OPENAI_API_KEY; echo
export OPENAI_API_KEY
crowcode
```

PowerShell:

```powershell
$key = Read-Host 'OpenAI API key' -AsSecureString
$env:OPENAI_API_KEY = [System.Net.NetworkCredential]::new('', $key).Password
Remove-Variable key
crowcode
```

Choose the provider/model in the TUI. Local-only configuration must be disabled
to use cloud models. Cloud calls require internet access and may incur charges.

## 5. Daily use and root flags

Examples work in Bash or PowerShell after installation:

```text
crowcode
crowcode -c "/path/to/project"
crowcode -p "Summarize this repository; do not modify files"
crowcode -p "Explain the failing test" -f json -q
crowcode -d
crowcode --help
```

On Windows use a native path such as `crowcode -c "C:\Projects\My Project"`.
For automation, redirect stdout (`> result.json`) and inspect the exit status
(`$?`/`$PIPESTATUS` in Bash, `$LASTEXITCODE` in PowerShell). Non-interactive
prompts automatically approve tool permissions: use only trusted prompts,
workspaces, and credentials, with a restricted OS account.

| Root flag | Meaning |
| --- | --- |
| `-h`, `--help` | Help |
| `-v`, `--version` | CrowCode version |
| `-c`, `--cwd PATH` | Project working directory |
| `-d`, `--debug` | Debug logging |
| `-p`, `--prompt TEXT` | One non-interactive prompt |
| `-f`, `--output-format text\|json` | Prompt output format; default `text` |
| `-q`, `--quiet` | Hide the non-interactive spinner |

Root flags are not universally inherited by subcommands. Use each command's
`--help`; for example RAG/MCP use their own `--cwd`/`--debug`, while `rg`/`fzf`
receive only their native flags. Change directory before using `rg` or `fzf`;
CrowCode's root `-c` is not a tool argument.

Useful TUI keys:

| Key | Action |
| --- | --- |
| `Enter` | Send prompt |
| `Ctrl+K` | Command palette, including local connection setup |
| `Ctrl+O` | Model selection |
| `Ctrl+G` | Model parameter setup |
| `Ctrl+P` | Prompt/skills/agent overrides |
| `Ctrl+N` | New chat session |
| `Ctrl+E` | External editor |
| `Esc` | Close dialogs |
| `Ctrl+C` | Quit |

Review proposed edits and shell permissions, then inspect your source-control
diff and run your project's tests.

## 6. Embedded ripgrep how-to and flags

```text
crowcode rg --help
crowcode rg -n "TODO" .
crowcode rg -i -g "*.go" "error" .
crowcode rg -F "literal.text" .
crowcode rg --files -g "*.md"
crowcode rg --hidden -g "!.git/**" "setting" .
```

| Native flag | Purpose |
| --- | --- |
| `-n` | Line numbers |
| `-i` | Case-insensitive search |
| `-F` | Literal text instead of regex |
| `-g GLOB` | Include/exclude filename glob; quote it |
| `-l` | Print matching filenames only |
| `--files` | List files rather than search contents |
| `--hidden` | Include hidden files |
| `--no-ignore` | Do not honor ignore files |
| `-A N`, `-B N`, `-C N` | Context after/before/around matches |
| `--json` | Structured search events |
| `-e PATTERN` | Explicit pattern, including one beginning with `-` |
| `--version`, `--help` | Version/full native reference |

By default ripgrep respects ignore files and skips hidden files. Exit codes:
`0` = match, `1` = no match, `2` = error. Quote Windows paths with spaces, e.g.
`crowcode rg -n "TODO" "C:\Projects\My Project"`.

## 7. Embedded fzf how-to and flags

Interactive file picker, on either platform:

```text
crowcode rg --files | crowcode fzf
```

Non-interactive filtering:

Linux:

```bash
printf 'alpha\nbeta\ngamma\n' | crowcode fzf --filter beta
crowcode rg --files -0 | crowcode fzf --read0 --print0 --filter README
```

Windows PowerShell:

```powershell
'alpha', 'beta', 'gamma' | crowcode fzf --filter beta
crowcode rg --files | crowcode fzf --query README
```

Use newline pipelines in Windows PowerShell 5.1; older PowerShell versions
transform native byte streams, so NUL-delimited/binary pipelines are not safe.
The NUL-delimited Bash example handles filenames containing newlines.

| Native flag | Purpose |
| --- | --- |
| `--query TEXT` | Initial interactive search |
| `--filter TEXT` | Filter without an interactive terminal |
| `--exact` | Exact matching |
| `--multi` | Multiple selections with Tab |
| `--height 40%` | Picker height |
| `--layout reverse` | Prompt at top |
| `--read0`, `--print0` | NUL-delimited input/output |
| `--preview COMMAND` | Preview using an external shell command |
| `--version`, `--help` | Version/full native reference |

fzf returns selected lines on stdout; Enter accepts and Esc cancels. Common exit
codes: `0` = selection/match, `1` = no match, `2` = error, `130` = interrupted.
Preview commands and `FZF_DEFAULT_COMMAND` need a suitable external shell and
any programs they call. CrowCode does not supply them.

## 8. RAG, MCP, setup, and completion reference

### RAG

Index and search project documentation (Linux or PowerShell):

```text
crowcode rag index --source-type directory --uri folder://project-docs --directory-path ./docs --recursive --title "Project Docs"
crowcode rag search --query "deployment" --limit 5
crowcode rag benchmark --query "deployment" --runs 10 --expected-term deploy
crowcode rag backfill-embeddings --limit 100
```

For Windows shares use
`--source-type remote_share --directory-path "\\server\share\docs"`;
on Linux mount the share first and use its mounted path. Run indexing and
searching against the same working/data directory.

- All RAG subcommands: `--cwd PATH`, `--debug`, `--help`.
- `index`: `--uri` is required; `--source-type` is `manual` (default),
  `local_file`, `web`, `sql`, `object_storage`, `directory`, or `remote_share`.
  Input flags: `--text`, `--text-file`, `--sql-dsn`, `--sql-query`, `--object-uri`,
  `--directory-path`. Metadata: `--name`, `--external-id`, `--title`.
  Bounds: `--recursive` (true), `--max-files` (500), `--max-bytes` (16777216),
  `--chunk-size` (1200), `--chunk-overlap` (200).
  `--embedding-model` defaults to the label `crowcode-hash-128`; the active
  embedding backend is configured separately under `rag.embeddings`.
- `search`: `--query`, `--limit` (5).
- `benchmark`: `--query`, `--limit` (5), `--runs` (10), `--expected-term`.
- `backfill-embeddings`: `--limit` (0 = all).

Ollama embeddings need the embedding model installed and its server running.
Inspect `crowcode rag index --help` for supported document formats.

### MCP

```text
crowcode mcp list
crowcode mcp list --source manifest,binary --type stdio
crowcode mcp list --cwd "/path/to/project"
```

Flags: `--source explicit,manifest,binary`, `--type stdio,sse`, `--cwd`,
`--debug`, `--help`. Built-in profiles are `caveman`, `coding`, and
`reverse_engineering`; their external server runtimes are not included.
For offline discovery, place trusted executables in
`<project>/.crowcode/mcp/bin` (with execute permission on Linux).
Manifests go in `.crowcode/mcp/servers`. Explicit `mcpServers` configuration
overrides manifests, which override binary discovery for matching names.

### Setup wizard

`crowcode setup --help` lists `--provider ollama|vllm|sglang`, `--endpoint`,
`--local-only`, `--prefer-ollama-embedder`, `--no-probe`, and `-y`/`--yes`.
**Known existing limitation:** standalone `crowcode setup` currently reports
`config not loaded`. Use the TUI wizard or JSON/environment configuration in
section 4 instead. This does not affect the bundled `rg`/`fzf` commands.

### Shell completion

`crowcode completion --help` describes generation for Bash, Zsh, Fish, and
PowerShell. Completion setup is optional and separate from bundled fzf shell
integration, which is not installed.

## 9. Deployment, offline operation, upgrades, and removal

- Deploy only the matching executable plus this guide and license notices.
  Do not run as root/Administrator unless your task genuinely requires it.
- For unattended prompts, set the working directory, credentials/endpoint,
  writable data directory, and temporary directory in the job environment.
  CrowCode is a CLI, not a daemon; use `-p`, `-q`, and an appropriate output
  format instead of starting the interactive TUI in a service.
- Linux containers can copy the Linux executable into an image. The CLI/tools
  do not require a system Go/Rust installation, but assistant shell operations
  need Bash and whichever tools the project uses. Mount writable data/temp
  paths and expose or route the model endpoint deliberately.
- Offline use requires model weights, optional embedding models, and any
  optional MCP/LSP/Python dependencies to be provisioned beforehand. Embedded
  rg/fzf never download anything at runtime. Avoid cloud providers and tools
  that fetch remote data if completely offline operation is required.
- Stop CrowCode before upgrading. Back up `.crowcode.json` and the whole
  `.crowcode/` directory while no process is writing to its SQLite database.
  Replace the executable, verify checksums and tool versions again, and retain
  a backup of the old binary/data for rollback. Database migrations can make
  downgrades unsafe without restoring the corresponding data backup.
- Uninstall by removing the installed executable/docs and the user `PATH`
  entry if you added it. Keep config/data unless you explicitly want to erase
  sessions and indexes. Do not remove unrelated tools from the system.

## 10. Troubleshooting and diagnostics

| Symptom | Action |
| --- | --- |
| Linux `Permission denied` | Check executable mode and mount `noexec` policy for both CrowCode and its temporary directory. |
| Tool extraction fails | Provide a writable, executable temp location: Linux `mkdir -p "$HOME/.cache/crowcode-tmp"; chmod 700 "$HOME/.cache/crowcode-tmp"; export TMPDIR="$HOME/.cache/crowcode-tmp"`; Windows set `TEMP`/`TMP` to an approved per-user directory. |
| Windows tool launch blocked | Check application-control/antivirus policy for CrowCode and its extracted executables; request a narrowly scoped approval from your administrator. |
| `crowcode` not found | Reopen the terminal after installation, inspect `PATH`, or invoke the executable by absolute path. |
| Shell tool fails on Windows | Configure Git Bash, not PowerShell/CMD, as described in section 4. |
| Shell cannot find rg/fzf | A login/profile script may reset `PATH`; use the non-login Bash arguments in section 4. Built-in searches and direct tool commands resolve bundled paths independently. |
| No models or connection refused | Start the model server, check endpoint/port, and ensure the requested model is installed. |
| No ripgrep results | Check ignore rules, hidden files, glob quoting, and search directory; exit 1 is not a failure. |
| fzf fails in automation | Use `--filter`; interactive selection requires a terminal. |
| Database errors | Check permissions/free space and the configured data directory. Back up data before repair. |
| Standalone setup fails | Use JSON/env settings or the TUI wizard; see section 8. |

For persistent diagnostic logs set `OPENCODE_DEV_DEBUG=true` in addition to
`--debug`; logs are written under the configured data directory. Logs and
stored sessions can contain prompts, source code, and credentials: inspect and
redact them before sharing.

An abrupt kill/crash can leave `crowcode-tools-*` directories in your temp
location. After verifying that no CrowCode process is using them, remove only
the stale directories owned by your user. Normal exits clean them automatically.

## 11. Rebuilding these distributions

The checked-in compressed assets are embedded automatically by normal Go
builds. Linux amd64 embeds only the Linux pair; Windows amd64 only the Windows
pair. Other build targets continue using external `rg`/`fzf` from `PATH`.
No runtime download or build-time fetching of the tool assets is required.

From the repository root, in Bash on a Go-equipped build host:

```bash
go test ./...
VERSION="$(git describe --tags --always)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/opencode-ai/opencode/internal/version.Version=$VERSION" \
  -o dist/crowcode-linux-amd64 ./main.go
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/opencode-ai/opencode/internal/version.Version=$VERSION" \
  -o dist/crowcode-windows-amd64.exe ./main.go
cp LICENSE dist/LICENSE.txt
cp internal/bundledtools/assets/THIRD_PARTY_NOTICES.txt dist/THIRD_PARTY_NOTICES.txt
(cd dist && sha256sum crowcode-linux-amd64 crowcode-windows-amd64.exe > SHA256SUMS)
```

Use a Go version satisfying `go.mod`; these artifacts were built with Go 1.26.5.
To update tools, obtain both platform releases from the upstream URLs in the
notices, verify archive SHA-256 digests against upstream release metadata,
extract the executables, and compress with `gzip -n -9` into the matching
`internal/bundledtools/assets/{rg,fzf}-{linux,windows}-amd64.gz` files.
Update versions/licenses/checksums/tests, rebuild **both** CrowCode executables,
and regenerate `SHA256SUMS`.

For broader feature documentation see the repository's
[documentation book](../docs/book/README.md), especially its user,
administrator, and tools guides.
