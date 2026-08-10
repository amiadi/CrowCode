# CrowCode Documentation Book

This documentation book helps teams and individual users get full value from CrowCode.

Platform support:
- CrowCode is intended for both Linux and Windows users across CLI, TUI, connectors, and tooling workflows.

Audience:
- Administrators: deployment, configuration, governance, operations.
- Users: daily workflows, prompt patterns, RAG usage, and tool usage.

## Start Here

- Administrator Guide: [Administrator Guide](./administrator-guide.md)
- User Guide: [User Guide](./user-guide.md)
- Tools Reference: [Tools Reference](./tools-reference.md)

## What You Can Do With CrowCode

- Run a local-first AI coding assistant in terminal and TUI.
- Use local or container-backed Ollama endpoints.
- Use a guided setup wizard for Ollama, vLLM, and SGLang endpoints.
- Use built-in MCP server profiles out of the box.
- Auto-discover MCP servers from JSON manifests or offline local binaries.
- Inspect effective MCP server sources with `crowcode mcp list` and filter by source/type.
- Configure agent behavior and runtime sampling controls.
- Add RAG sources from files, folders, remote shares, web, SQL, and object-like paths.
- Register Python script tools with permission controls.
- Persist sessions and RAG artifacts in SQLite for reproducible workflows.

## Recommended Learning Path

1. Read the User Guide quickstart and run basic chat workflows.
2. Learn RAG ingestion patterns from the User Guide.
3. Learn operational controls from the Administrator Guide.
4. Use the Tools Reference as your lookup during day-to-day usage.

## Project Terms

- Workspace: Current working directory for CrowCode operations.
- Session: A chat history tracked in SQLite.
- RAG: Retrieval-augmented context built from indexed content.
- Connector: Ingestion path for RAG index operations.
- Tool: Callable capability used by the assistant (shell, edit, grep, python, etc.).
