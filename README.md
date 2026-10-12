# CrowCode

CrowCode is a customized distribution of CrowCode focused on local-first workflows,
including local or container-backed Ollama usage and SQLite-backed RAG.

Note:
Cross-platform support target: Linux and Windows are both first-class environments for CrowCode features and tools.

This repository is no longer maintained and has been archived for provenance.

The project has continued under the name [Crush][crush], developed by the original author and the Charm team.

Please follow [Crush][crush] for ongoing development.

[crush]: https://github.com/charmbracelet/crush


# ⌬ CrowCode

<p align="center"><img src="https://github.com/user-attachments/assets/9ae61ef6-70e5-4876-bc45-5bcb4e52c714" width="800"></p>

> **⚠️ Early Development Notice:** This project is in early development and is not yet ready for production use. Features may change, break, or be incomplete. Use at your own risk.

A powerful terminal-based AI assistant for developers, providing intelligent coding assistance directly in your terminal.

## Overview

CrowCode is a Go-based CLI application that brings AI assistance to your terminal. It provides a TUI (Terminal User Interface) for interacting with various AI models to help with coding tasks, debugging, and more.

<p>For a quick video overview, check out
<a href="https://www.youtube.com/watch?v=P8luPmEa1QI"><img width="25" src="https://upload.wikimedia.org/wikipedia/commons/0/09/YouTube_full-color_icon_%282017%29.svg"> CrowCode + Gemini 2.5 Pro: BYE Claude Code! I'm SWITCHING To the FASTEST AI Coder!</a></p>

<a href="https://www.youtube.com/watch?v=P8luPmEa1QI"><img width="550" src="https://i3.ytimg.com/vi/P8luPmEa1QI/maxresdefault.jpg"></a><p>

## Features

- **Interactive TUI**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a smooth terminal experience
- **Multiple AI Providers**: Support for OpenAI, Anthropic Claude, Google Gemini, AWS Bedrock, Groq, Azure OpenAI, OpenRouter, xAI, GitHub Copilot, and self-hosted OpenAI-compatible endpoints
- **Session Management**: Save and manage multiple conversation sessions
- **Session Summaries and Auto-Compaction**: Summarize long conversations and retain structured task state so work can continue with context
- **Tool Integration**: AI can execute commands, search files, and modify code
- **MCP Integration**: Use built-in profiles and automatically discover MCP servers from manifests or local binaries
- **SQLite-backed RAG**: Index and search local files, web pages, SQL results, object storage, folders, and remote shares; optionally add retrieved context to chat
- **Local Model Setup**: Configure Ollama, vLLM, or SGLang endpoints with a guided wizard, local-only mode, and per-agent model parameters
- **Python Script Tools**: Expose configured scripts to the assistant with execution timeouts and permission checks
- **Non-interactive CLI**: Run prompts from scripts and choose plain-text or JSON output
- **Vim-like Editor**: Integrated editor with text input capabilities
- **Persistent Storage**: SQLite database for storing conversations and sessions
- **LSP Integration**: Language Server Protocol support for code intelligence
- **File Change Tracking**: Track and visualize file changes during sessions
- **External Editor Support**: Open your preferred editor for composing messages
- **Prompt and Command Customization**: Add workspace prompt overrides and custom commands with named arguments

## RAG Connector Quickstart

Use `rag index` to ingest local folders or mounted remote shares with recursive traversal.

Index a local folder recursively:

```bash
crowcode rag index \
  --source-type directory \
  --uri folder://project-docs \
  --directory-path ./docs \
  --recursive \
  --max-files 1000 \
  --max-bytes 33554432 \
  --title "Project Docs" \
  --external-id project-docs-v1
```

Index a Linux mounted share recursively:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://linux-team \
  --directory-path /mnt/team-share/knowledge \
  --recursive \
  --title "Linux Team Share" \
  --external-id linux-team-share-v1
```

Index a Windows UNC share recursively:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://windows-team \
  --directory-path "\\\\server\\share\\knowledge" \
  --recursive \
  --title "Windows Team Share" \
  --external-id windows-team-share-v1
```

Common connector flags:
- `--directory-path` folder or share path (`/mnt/...`, `\\server\\share\\...`, `file://`, `smb://`)
- `--recursive` recurse nested folders (default: `true`)
- `--max-files` max files to ingest (default: `500`)
- `--max-bytes` max total bytes to ingest (default: `16777216`)

## Documentation Book

For complete onboarding and advanced usage, use the project documentation book:

- Book Home: [docs/book/README.md](docs/book/README.md)
- Administrator Guide: [docs/book/administrator-guide.md](docs/book/administrator-guide.md)
- User Guide: [docs/book/user-guide.md](docs/book/user-guide.md)
- Tools Reference: [docs/book/tools-reference.md](docs/book/tools-reference.md)

## Built-in MCP Servers and Auto-Discovery

CrowCode ships with built-in MCP profiles and supports auto-discovery for user-installed MCP servers.

Built-in profiles available out of the box:
- `caveman`
- `coding`
- `reverse_engineering`

CrowCode auto-loads MCP servers from:
- `<data.directory>/mcp/servers/builtin`
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

### Offline local binary discovery (no JSON manifests required)

Users can drop pre-downloaded MCP server binaries directly into discovery folders and CrowCode will auto-register them as `stdio` MCP servers.

Binary discovery folders:
- `<data.directory>/mcp/bin`
- `$XDG_CONFIG_HOME/crowcode/mcp/bin`
- `$HOME/.crowcode/mcp/bin`
- `<workspace>/.crowcode/mcp/bin`

Compatibility folders also scanned for binaries:
- `$XDG_CONFIG_HOME/crowcode/mcp/servers`
- `$HOME/.crowcode/mcp/servers`
- `<workspace>/.crowcode/mcp/servers`

Example (offline binary drop-in):

```bash
mkdir -p ./.crowcode/mcp/bin
cp ./downloads/my-mcp-server ./.crowcode/mcp/bin/
chmod +x ./.crowcode/mcp/bin/my-mcp-server
crowcode
```

The server is auto-registered with a normalized name derived from the file name (for example `my-mcp-server` -> `my_mcp_server`).

Priority behavior:
- Explicit `mcpServers` configuration overrides manifest discovery; manifest discovery overrides binary discovery for matching names.

Inspect effective MCP servers and their source:

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

This command shows, for each effective MCP server:
- server name
- source (`explicit`, `manifest`, `binary`)
- type (`stdio` or `sse`)
- endpoint/command
- origin file path or runtime path

## Installation

### Online Install (Linux and macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/crowcode-ai/crowcode/main/install | bash
```

### Offline Install (Linux and Windows)

Linux:

```bash
tar -xzf crowcode-linux-amd64.tar.gz
chmod +x crowcode
./crowcode -v
```

Windows PowerShell:

```powershell
Expand-Archive .\crowcode-windows-amd64.zip -DestinationPath .\crowcode
.\crowcode\crowcode.exe -v
```

### Online Clone and Build

```bash
git clone https://github.com/crowcode-ai/crowcode.git
cd crowcode
go build -o crowcode
./crowcode -v
```

### Offline Clone and Build

Create a bundle from an online machine:

```bash
git clone https://github.com/crowcode-ai/crowcode.git
cd crowcode
git bundle create crowcode.bundle --all
```

Import and build on an offline machine:

```bash
git clone crowcode.bundle crowcode
cd crowcode
go build -o crowcode
./crowcode -v
```

### Linux and Windows Cross-Compilation

Linux:

```bash
GOOS=linux GOARCH=amd64 go build -o dist/crowcode-linux-amd64
GOOS=linux GOARCH=arm64 go build -o dist/crowcode-linux-arm64
```

Windows:

```bash
GOOS=windows GOARCH=amd64 go build -o dist/crowcode-windows-amd64.exe
GOOS=windows GOARCH=arm64 go build -o dist/crowcode-windows-arm64.exe
```

### Package Managers (upstream crowcode ecosystem)

```bash
brew install crowcode-ai/tap/crowcode
```

```bash
yay -S crowcode-ai-bin
```

```bash
paru -S crowcode-ai-bin
```

### Using Go install

```bash
go install github.com/crowcode-ai/crowcode@latest
```

When using `go install`, build or rename the binary to `crowcode` for CrowCode CLI naming.

## Configuration

- `$HOME/.crowcode.json`
- `$XDG_CONFIG_HOME/crowcode/.crowcode.json`
- `./.crowcode.json` (local directory)

### Connection Setup Wizard (CLI + TUI)

CrowCode includes a guided setup wizard for local OpenAI-compatible endpoints:
- Ollama (default `http://127.0.0.1:11434`)
- vLLM (default `http://127.0.0.1:8000`)
- SGLang (default `http://127.0.0.1:30000`)

Run the CLI wizard:

```bash
crowcode setup
```

Non-interactive example:

```bash
crowcode setup \
  --yes \
  --provider ollama \
  --endpoint http://192.168.1.25:11434 \
  --local-only \
  --prefer-ollama-embedder
```

Open from TUI:
1. Start `crowcode`
2. Press `Ctrl+K`
3. Select `Setup Local Connection Wizard`

Wizard output updates config values such as `localEndpoint`, `localOnly`, and local provider defaults.

Setup flags reference:
- `--provider`: `ollama`, `vllm`, or `sglang`
- `--endpoint`: server URL such as `http://127.0.0.1:11434`
- `--local-only`: disable non-local providers for this config
- `--prefer-ollama-embedder`: set RAG embeddings provider to `ollama`
- `--no-probe`: skip endpoint connectivity probe
- `--yes`: non-interactive mode using only provided flags

### Auto Compact Feature

CrowCode includes an auto compact feature that automatically summarizes your conversation when it approaches the model's context window limit. When enabled (default setting), this feature:

- Monitors token usage during your conversation
- Automatically triggers summarization when usage reaches 95% of the model's context window
- Creates a new session with the summary, allowing you to continue your work without losing context
- Helps prevent "out of context" errors that can occur with long conversations

You can enable or disable this feature in your configuration file:

```json
{
  "autoCompact": true // default is true
}
```

### Environment Variables

You can configure CrowCode using environment variables:

| Environment Variable       | Purpose                                                                          |
| -------------------------- | -------------------------------------------------------------------------------- |
| `ANTHROPIC_API_KEY`        | For Claude models                                                                |
| `OPENAI_API_KEY`           | For OpenAI models                                                                |
| `GEMINI_API_KEY`           | For Google Gemini models                                                         |
| `GITHUB_TOKEN`             | For Github Copilot models (see [Using Github Copilot](#using-github-copilot))    |
| `VERTEXAI_PROJECT`         | For Google Cloud VertexAI (Gemini)                                               |
| `VERTEXAI_LOCATION`        | For Google Cloud VertexAI (Gemini)                                               |
| `GROQ_API_KEY`             | For Groq models                                                                  |
| `AWS_ACCESS_KEY_ID`        | For AWS Bedrock (Claude)                                                         |
| `AWS_SECRET_ACCESS_KEY`    | For AWS Bedrock (Claude)                                                         |
| `AWS_REGION`               | For AWS Bedrock (Claude)                                                         |
| `AZURE_OPENAI_ENDPOINT`    | For Azure OpenAI models                                                          |
| `AZURE_OPENAI_API_KEY`     | For Azure OpenAI models (optional when using Entra ID)                           |
| `AZURE_OPENAI_API_VERSION` | For Azure OpenAI models                                                          |
| `LOCAL_ENDPOINT`           | For self-hosted models                                                           |
| `SHELL`                    | Default shell to use (if not specified in config)                                |

### Shell Configuration

CrowCode allows you to configure the shell used by the bash tool. By default, it uses the shell specified in the `SHELL` environment variable, or falls back to `/bin/bash` if not set.

You can override this in your configuration file:

```json
{
  "shell": {
    "path": "/bin/zsh",
    "args": ["-l"]
  }
}
```

This is useful if you want to use a different shell than your default system shell, or if you need to pass specific arguments to the shell.

### Configuration File Structure

```json
{
  "data": {
    "directory": ".crowcode"
  },
  "providers": {
    "openai": {
      "apiKey": "your-api-key",
      "disabled": false
    },
    "anthropic": {
      "apiKey": "your-api-key",
      "disabled": false
    },
    "copilot": {
      "disabled": false
    },
    "groq": {
      "apiKey": "your-api-key",
      "disabled": false
    },
    "openrouter": {
      "apiKey": "your-api-key",
      "disabled": false
    }
  },
  "agents": {
    "coder": {
      "model": "claude-3.7-sonnet",
      "maxTokens": 5000
    },
    "task": {
      "model": "claude-3.7-sonnet",
      "maxTokens": 5000
    },
    "title": {
      "model": "claude-3.7-sonnet",
      "maxTokens": 80
    }
  },
  "shell": {
    "path": "/bin/bash",
    "args": ["-l"]
  },
  "mcpServers": {
    "example": {
      "type": "stdio",
      "command": "path/to/mcp-server",
      "env": [],
      "args": []
    }
  },
  "lsp": {
    "go": {
      "disabled": false,
      "command": "gopls"
    }
  },
  "debug": false,
  "debugLSP": false,
  "autoCompact": true
}
```

## Supported AI Models

CrowCode supports a variety of AI models from different providers:

### OpenAI

- GPT-4.1 family (gpt-4.1, gpt-4.1-mini, gpt-4.1-nano)
- GPT-4.5 Preview
- GPT-4o family (gpt-4o, gpt-4o-mini)
- O1 family (o1, o1-pro, o1-mini)
- O3 family (o3, o3-mini)
- O4 Mini

### Anthropic

- Claude 4 Sonnet
- Claude 4 Opus
- Claude 3.5 Sonnet
- Claude 3.5 Haiku
- Claude 3.7 Sonnet
- Claude 3 Haiku
- Claude 3 Opus

### GitHub Copilot

- GPT-3.5 Turbo
- GPT-4
- GPT-4o
- GPT-4o Mini
- GPT-4.1
- Claude 3.5 Sonnet
- Claude 3.7 Sonnet
- Claude 3.7 Sonnet Thinking
- Claude Sonnet 4
- O1
- O3 Mini
- O4 Mini
- Gemini 2.0 Flash
- Gemini 2.5 Pro

### Google

- Gemini 2.5
- Gemini 2.5 Flash
- Gemini 2.0 Flash
- Gemini 2.0 Flash Lite

### AWS Bedrock

- Claude 3.7 Sonnet

### Groq

- Llama 4 Maverick (17b-128e-instruct)
- Llama 4 Scout (17b-16e-instruct)
- QWEN QWQ-32b
- Deepseek R1 distill Llama 70b
- Llama 3.3 70b Versatile

### Azure OpenAI

- GPT-4.1 family (gpt-4.1, gpt-4.1-mini, gpt-4.1-nano)
- GPT-4.5 Preview
- GPT-4o family (gpt-4o, gpt-4o-mini)
- O1 family (o1, o1-mini)
- O3 family (o3, o3-mini)
- O4 Mini

### Google Cloud VertexAI

- Gemini 2.5
- Gemini 2.5 Flash

### OpenRouter

- Routes supported OpenAI, Anthropic, and Gemini model families through OpenRouter
- Includes DeepSeek R1 Free

### xAI

- Grok 3 Beta
- Grok 3 Mini Beta
- Grok 3 Fast Beta
- Grok 3 Mini Fast Beta

## Usage

```bash
# Start CrowCode
crowcode

# Start with debug logging
crowcode -d

# Start with a specific working directory
crowcode -c /path/to/project
```

## Non-interactive Prompt Mode

You can run CrowCode in non-interactive mode by passing a prompt directly as a command-line argument. This is useful for scripting, automation, or when you want a quick answer without launching the full TUI.

```bash
# Run a single prompt and print the AI's response to the terminal
crowcode -p "Explain the use of context in Go"

# Get response in JSON format
crowcode -p "Explain the use of context in Go" -f json

# Run without showing the spinner (useful for scripts)
crowcode -p "Explain the use of context in Go" -q
```

In this mode, CrowCode will process your prompt, print the result to standard output, and then exit. All permissions are auto-approved for the session.

By default, a spinner animation is displayed while the model is processing your query. You can disable this spinner with the `-q` or `--quiet` flag, which is particularly useful when running CrowCode from scripts or automated workflows.

### Output Formats

CrowCode supports the following output formats in non-interactive mode:

| Format | Description                     |
| ------ | ------------------------------- |
| `text` | Plain text output (default)     |
| `json` | Output wrapped in a JSON object |

The output format is implemented as a strongly-typed `OutputFormat` in the codebase, ensuring type safety and validation when processing outputs.

## Command-line Flags

| Flag              | Short | Description                                         |
| ----------------- | ----- | --------------------------------------------------- |
| `--help`          | `-h`  | Display help information                            |
| `--debug`         | `-d`  | Enable debug mode                                   |
| `--cwd`           | `-c`  | Set current working directory                       |
| `--prompt`        | `-p`  | Run a single prompt in non-interactive mode         |
| `--output-format` | `-f`  | Output format for non-interactive mode (text, json) |
| `--quiet`         | `-q`  | Hide spinner in non-interactive mode                |

## Keyboard Shortcuts

### Global Shortcuts

| Shortcut | Action                                                  |
| -------- | ------------------------------------------------------- |
| `Ctrl+C` | Quit application                                        |
| `Ctrl+?` | Toggle help dialog                                      |
| `?`      | Toggle help dialog (when not in editing mode)           |
| `Ctrl+L` | View logs                                               |
| `Ctrl+A` | Switch session                                          |
| `Ctrl+K` | Command dialog                                          |
| `Ctrl+O` | Toggle model selection dialog                           |
| `Esc`    | Close current overlay/dialog or return to previous mode |

### Chat Page Shortcuts

| Shortcut | Action                                  |
| -------- | --------------------------------------- |
| `Ctrl+N` | Create new session                      |
| `Ctrl+X` | Cancel current operation/generation     |
| `i`      | Focus editor (when not in writing mode) |
| `Esc`    | Exit writing mode and focus messages    |

### Editor Shortcuts

| Shortcut            | Action                                    |
| ------------------- | ----------------------------------------- |
| `Ctrl+S`            | Send message (when editor is focused)     |
| `Enter` or `Ctrl+S` | Send message (when editor is not focused) |
| `Ctrl+E`            | Open external editor                      |
| `Esc`               | Blur editor and focus messages            |

### Session Dialog Shortcuts

| Shortcut   | Action           |
| ---------- | ---------------- |
| `↑` or `k` | Previous session |
| `↓` or `j` | Next session     |
| `Enter`    | Select session   |
| `Esc`      | Close dialog     |

### Model Dialog Shortcuts

| Shortcut   | Action            |
| ---------- | ----------------- |
| `↑` or `k` | Move up           |
| `↓` or `j` | Move down         |
| `←` or `h` | Previous provider |
| `→` or `l` | Next provider     |
| `Esc`      | Close dialog      |

### Permission Dialog Shortcuts

| Shortcut                | Action                       |
| ----------------------- | ---------------------------- |
| `←` or `left`           | Switch options left          |
| `→` or `right` or `tab` | Switch options right         |
| `Enter` or `space`      | Confirm selection            |
| `a`                     | Allow permission             |
| `A`                     | Allow permission for session |
| `d`                     | Deny permission              |

### Logs Page Shortcuts

| Shortcut           | Action              |
| ------------------ | ------------------- |
| `Backspace` or `q` | Return to chat page |

## AI Assistant Tools

CrowCode's AI assistant has access to various tools to help with coding tasks:

### File and Code Tools

| Tool          | Description                 | Parameters                                                                               |
| ------------- | --------------------------- | ---------------------------------------------------------------------------------------- |
| `glob`        | Find files by pattern       | `pattern` (required), `path` (optional)                                                  |
| `grep`        | Search file contents        | `pattern` (required), `path` (optional), `include` (optional), `literal_text` (optional) |
| `ls`          | List directory contents     | `path` (optional), `ignore` (optional array of patterns)                                 |
| `view`        | View file contents          | `file_path` (required), `offset` (optional), `limit` (optional)                          |
| `write`       | Write to files              | `file_path` (required), `content` (required)                                             |
| `edit`        | Edit files                  | Various parameters for file editing                                                      |
| `patch`       | Apply patches to files      | `file_path` (required), `diff` (required)                                                |
| `diagnostics` | Get diagnostics information | `file_path` (optional)                                                                   |

### Other Tools

| Tool          | Description                            | Parameters                                                                                |
| ------------- | -------------------------------------- | ----------------------------------------------------------------------------------------- |
| `bash`        | Execute shell commands                 | `command` (required), `timeout` (optional)                                                |
| `fetch`       | Fetch data from URLs                   | `url` (required), `format` (required), `timeout` (optional)                               |
| `sourcegraph` | Search code across public repositories | `query` (required), `count` (optional), `context_window` (optional), `timeout` (optional) |
| `agent`       | Run sub-tasks with the AI agent        | `prompt` (required)                                                                       |

## Architecture

CrowCode is built with a modular architecture:

- **cmd**: Command-line interface using Cobra
- **internal/app**: Core application services
- **internal/config**: Configuration management
- **internal/db**: Database operations and migrations
- **internal/llm**: LLM providers and tools integration
- **internal/tui**: Terminal UI components and layouts
- **internal/logging**: Logging infrastructure
- **internal/message**: Message handling
- **internal/session**: Session management
- **internal/lsp**: Language Server Protocol integration

## Custom Commands

CrowCode supports custom commands that can be created by users to quickly send predefined prompts to the AI assistant.

### Creating Custom Commands

Custom commands are predefined prompts stored as Markdown files in one of three locations:

1. **User Commands** (prefixed with `user:`):

   ```
   $XDG_CONFIG_HOME/crowcode/commands/
   ```

   (typically `~/.config/crowcode/commands/` on Linux/macOS)

   or

   ```
   $HOME/.crowcode/commands/
   ```

2. **Project Commands** (prefixed with `project:`):

   ```
   <PROJECT DIR>/.crowcode/commands/
   ```

Each `.md` file in these directories becomes a custom command. The file name (without extension) becomes the command ID.

For example, creating a file at `~/.config/crowcode/commands/prime-context.md` with content:

```markdown
RUN git ls-files
READ README.md
```

This creates a command called `user:prime-context`.

### Command Arguments

CrowCode supports named arguments in custom commands using placeholders in the format `$NAME` (where NAME consists of uppercase letters, numbers, and underscores, and must start with a letter).

For example:

```markdown
# Fetch Context for Issue $ISSUE_NUMBER

RUN gh issue view $ISSUE_NUMBER --json title,body,comments
RUN git grep --author="$AUTHOR_NAME" -n .
RUN grep -R "$SEARCH_PATTERN" $DIRECTORY
```

When you run a command with arguments, CrowCode will prompt you to enter values for each unique placeholder. Named arguments provide several benefits:

- Clear identification of what each argument represents
- Ability to use the same argument multiple times
- Better organization for commands with multiple inputs

### Organizing Commands

You can organize commands in subdirectories:

```
~/.config/crowcode/commands/git/commit.md
```

This creates a command with ID `user:git:commit`.

### Using Custom Commands

1. Press `Ctrl+K` to open the command dialog
2. Select your custom command (prefixed with either `user:` or `project:`)
3. Press Enter to execute the command

The content of the command file will be sent as a message to the AI assistant.

### Built-in Commands

CrowCode includes several built-in commands:

| Command            | Description                                                                                         |
| ------------------ | --------------------------------------------------------------------------------------------------- |
| Initialize Project | Creates or updates the CrowCode.md memory file with project-specific information                    |
| Compact Session    | Manually triggers the summarization of the current session, creating a new session with the summary |

## MCP (Model Context Protocol)

CrowCode implements the Model Context Protocol (MCP) to extend its capabilities through external tools. MCP provides a standardized way for the AI assistant to interact with external services and tools.

### MCP Features

- **External Tool Integration**: Connect to external tools and services via a standardized protocol
- **Tool Discovery**: Automatically discover available tools from MCP servers
- **Multiple Connection Types**:
  - **Stdio**: Communicate with tools via standard input/output
  - **SSE**: Communicate with tools via Server-Sent Events
- **Security**: Permission system for controlling access to MCP tools

### Configuring MCP Servers

MCP servers are defined in the configuration file under the `mcpServers` section:

```json
{
  "mcpServers": {
    "example": {
      "type": "stdio",
      "command": "path/to/mcp-server",
      "env": [],
      "args": []
    },
    "web-example": {
      "type": "sse",
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer token"
      }
    }
  }
}
```

### MCP Tool Usage

Once configured, MCP tools are automatically available to the AI assistant alongside built-in tools. They follow the same permission model as other tools, requiring user approval before execution.

## LSP (Language Server Protocol)

CrowCode integrates with Language Server Protocol to provide code intelligence features across multiple programming languages.

### LSP Features

- **Multi-language Support**: Connect to language servers for different programming languages
- **Diagnostics**: Receive error checking and linting information
- **File Watching**: Automatically notify language servers of file changes

### Configuring LSP

Language servers are configured in the configuration file under the `lsp` section:

```json
{
  "lsp": {
    "go": {
      "disabled": false,
      "command": "gopls"
    },
    "typescript": {
      "disabled": false,
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  }
}
```

### LSP Integration with AI

The AI assistant can access LSP features through the `diagnostics` tool, allowing it to:

- Check for errors in your code
- Suggest fixes based on diagnostics

While the LSP client implementation supports the full LSP protocol (including completions, hover, definition, etc.), currently only diagnostics are exposed to the AI assistant.

## Using Github Copilot

_Copilot support is currently experimental._

### Requirements
- [Copilot chat in the IDE](https://github.com/settings/copilot) enabled in GitHub settings
- One of:
  - VSCode Github Copilot chat extension
  - Github `gh` CLI
  - Neovim Github Copilot plugin (`copilot.vim` or `copilot.lua`)
  - Github token with copilot permissions

If using one of the above plugins or cli tools, make sure you use the authenticate
the tool with your github account. This should create a github token at one of the following locations:
- ~/.config/github-copilot/[hosts,apps].json
- $XDG_CONFIG_HOME/github-copilot/[hosts,apps].json

If using an explicit github token, you may either set the $GITHUB_TOKEN environment variable or add it to the crowcode.json config file at `providers.copilot.apiKey`.

## Using a self-hosted model provider

CrowCode can also load and use models from a self-hosted (OpenAI-like) provider.
This is useful for developers who want to experiment with custom models.

### Configuring a self-hosted provider

You can use a self-hosted model by setting the `LOCAL_ENDPOINT` environment variable.
This will cause CrowCode to load and use the models from the specified endpoint.

```bash
LOCAL_ENDPOINT=http://localhost:1235/v1
```

### Configuring a self-hosted model

You can also configure a self-hosted model in the configuration file under the `agents` section:

```json
{
  "agents": {
    "coder": {
      "model": "local.granite-3.3-2b-instruct@q8_0",
      "reasoningEffort": "high"
    }
  }
}
```

### Strict local-only Ollama example

If you want to force CrowCode to use only local or container-backed Ollama models,
enable `localOnly` and set per-agent runtime controls:

```json
{
  "localOnly": true,
  "agents": {
    "coder": {
      "model": "local.<your-ollama-model-id>",
      "maxTokens": 8192,
      "reasoningEffort": "high",
      "temperature": 0.2,
      "topP": 0.9,
      "minP": 0.05,
      "topK": 40,
      "numCtx": 32768,
      "repeatPenalty": 1.1,
      "seed": 42
    },
    "summarizer": {
      "model": "local.<your-ollama-model-id>",
      "maxTokens": 2048,
      "reasoningEffort": "medium",
      "temperature": 0.1,
      "topP": 0.8,
      "minP": 0.03,
      "topK": 30,
      "numCtx": 32768,
      "repeatPenalty": 1.05,
      "seed": 42
    },
    "task": {
      "model": "local.<your-ollama-model-id>",
      "maxTokens": 4096,
      "reasoningEffort": "medium",
      "temperature": 0.2,
      "topP": 0.9,
      "minP": 0.05,
      "topK": 40,
      "numCtx": 32768,
      "repeatPenalty": 1.1,
      "seed": 42
    }
  }
}
```

With `localOnly: true`, CrowCode disables non-local providers at validation time and
forces all core agents to a discovered local model.

### Chat-time RAG augmentation

CrowCode can inject retrieved SQLite context into chat-time prompts before model generation.

```json
{
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
  }
}
```

Notes:
- `provider: "ollama"` uses `LOCAL_ENDPOINT` for embedding requests.
- Supported endpoint shapes include `/api/embed`, `/api/embeddings`, and `/v1/embeddings`.
- Set `provider` to `hash` for an offline fallback embedder.

### RAG index connectors and recursive folder ingestion

CrowCode supports multiple ingestion connectors through `rag index`.

Supported source types:
- `manual` and `local_file` via `--text` or `--text-file`
- `web` via `--uri` (http/https)
- `sql` via `--sql-dsn` and `--sql-query`
- `object_storage` via `--object-uri` (file path, `file://`, or presigned `https://`)
- `directory`, `folder`, `local_folder`, `remote_share` via recursive folder ingestion

Supported offline extracted file formats for `--text-file`, local `object_storage` paths, and directory/share ingestion:
- plain text and markup: `txt`, `md`, `markdown`, `rst`, `json`, `yaml`, `yml`, `xml`, `html`, `htm`
- tabular: `csv`, `tsv`, `xlsx`
- documents: `docx`, `pdf`

Directory and remote share flags:
- `--directory-path` folder/share path to ingest
- `--recursive` recurse nested folders (default: true)
- `--max-files` safety limit for number of files (default: 500)
- `--max-bytes` safety limit for total bytes read (default: 16777216)

Path compatibility for folder/share connector:
- Linux local or mounted share path, for example `/mnt/team-share/docs`
- Windows UNC style path, for example `\\server\\share\\docs`
- `file://` URI form
- `smb://` URI form

Examples:

Index local folder recursively:

```bash
crowcode rag index \
  --source-type directory \
  --uri folder://docs \
  --directory-path ./docs \
  --recursive \
  --max-files 1000 \
  --max-bytes 33554432 \
  --title "Docs Folder" \
  --external-id docs-folder-v1
```

Index Linux mounted remote share recursively:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://linux-team \
  --directory-path /mnt/team-share/knowledge \
  --recursive \
  --title "Team Share" \
  --external-id team-share-v1
```

Index Windows UNC remote share recursively:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://windows-team \
  --directory-path "\\\\server\\share\\knowledge" \
  --recursive \
  --title "Windows Team Share" \
  --external-id windows-team-share-v1
```

Index share using URI form:

```bash
crowcode rag index \
  --source-type remote_share \
  --uri share://smb-team \
  --directory-path smb://server/share/knowledge \
  --recursive \
  --title "SMB Team Share" \
  --external-id smb-team-share-v1
```

Operational notes:
- Folder connector ingests readable supported files and skips unsupported/binary files.
- Each ingested file is prefixed in the indexed payload for provenance (`# file: <relative_path>`).
- Use `rag backfill-embeddings` after changing embedding backend/model to re-embed existing chunks.

### Python script tools

You can register allowlisted Python scripts as callable agent tools.

```json
{
  "python": {
    "enabled": true,
    "interpreter": "python3",
    "defaultTimeoutMs": 60000,
    "requirePermission": true,
    "tools": {
      "summarize_repo": {
        "path": "./scripts/summarize_repo.py",
        "args": ["--format", "json"],
        "timeoutMs": 30000,
        "requirePermission": true,
        "env": {
          "PYTHONUNBUFFERED": "1"
        }
      }
    }
  }
}
```

Registered tools appear as `python.<tool_name>` and run with per-execution permission checks by default.

### Structured mission compaction

Summarization now persists two artifacts:
- A human-readable mission summary message.
- A machine-readable `summary_state` JSON payload stored on the session record.

The structured summary captures:
- conversation summary
- current focus
- decisions
- completed tasks
- pending tasks
- risks
- next action
- files touched

## Development

### Prerequisites

- Go 1.24.0 or higher

### Building from Source

```bash
# Clone the repository
git clone https://github.com/crowcode-ai/crowcode.git
cd crowcode

# Build
go build -o crowcode

# Run
./crowcode
```

## Acknowledgments

CrowCode gratefully acknowledges the contributions and support from these key individuals:

- [@isaacphi](https://github.com/isaacphi) - For the [mcp-language-server](https://github.com/isaacphi/mcp-language-server) project which provided the foundation for our LSP client implementation
- [@adamdottv](https://github.com/adamdottv) - For the design direction and UI/UX architecture

Special thanks to the broader open source community whose tools and libraries have made this project possible.

## License

CrowCode is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Here's how you can contribute:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please make sure to update tests as appropriate and follow the existing code style.
