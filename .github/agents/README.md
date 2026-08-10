# OpenCode Personal Customization Playbook

This README is the centralized operational guide for any agent that will customize this repository for a personal, local-first AI workflow.
It is optimized for precision, continuity, and efficient execution.

## 1) Mission

Target end-state:
- Ollama-only runtime (local process or container service) with complete runtime parameter control.
- SQLite-backed indexing and retrieval layer supporting local and network data sources for RAG.
- Config-driven extension pipeline for skills, tools, MCP servers, and Python script tools.
- High-quality compacting and summarization that keeps conversations focused on mission progress.

Primary custom agent:
- [.github/agents/opencode-personal-customizer.agent.md](.github/agents/opencode-personal-customizer.agent.md)

## 1.1) Locked Decisions

These decisions are fixed and should not be re-opened unless the user explicitly changes them:
- Embeddings strategy: support both Ollama embeddings and pluggable embedding backends.
- Network source scope for Phase 2: all first-class categories (web pages, SQL databases, object storage).
- Python tool security model: both allowlist-by-path and per-execution permission prompts.
- Retrieval strategy: hybrid from first implementation, optimized for strong performance.

## 2) Current Codebase Baseline (Evidence)

### 2.1 Local model/provider path already exists
- Local provider is recognized as `ProviderLocal` in [internal/llm/models/local.go](internal/llm/models/local.go).
- Local models are discovered from `LOCAL_ENDPOINT` (`/api/v0/models` and `/v1/models`) in [internal/llm/models/local.go](internal/llm/models/local.go).
- Local provider dispatch uses OpenAI-compatible transport with `WithOpenAIBaseURL(os.Getenv("LOCAL_ENDPOINT"))` in [internal/llm/provider/provider.go](internal/llm/provider/provider.go).

Implication:
- Any Ollama integration should be compatible with OpenAI-style endpoint mode first, then optionally enhanced for Ollama-native options.

### 2.2 Agent/provider configuration path exists
- Agent config supports `Model`, `MaxTokens`, `ReasoningEffort` in [internal/config/config.go](internal/config/config.go).
- Provider objects and enable/disable state are managed in [internal/config/config.go](internal/config/config.go).
- Provider selection is performed in `createAgentProvider` in [internal/llm/agent/agent.go](internal/llm/agent/agent.go).

Implication:
- The correct place to add model runtime controls is config + provider option plumbing.

### 2.3 SQLite persistence already exists
- SQLite connection and pragmas are configured in [internal/db/connect.go](internal/db/connect.go).
- Existing tables support sessions/messages/files only (generated SQLC files under [internal/db](internal/db)).

Implication:
- RAG indexing can be added using new SQL migrations and SQLC queries.

### 2.4 Compaction exists but is generic
- Summarizer prompt is generic in [internal/llm/prompt/summarizer.go](internal/llm/prompt/summarizer.go).
- Session summary linkage exists (`SummaryMessageID`) through [internal/session/session.go](internal/session/session.go) and [internal/llm/agent/agent.go](internal/llm/agent/agent.go).
- Auto-compact trigger exists in TUI flow in [internal/tui/tui.go](internal/tui/tui.go).

Implication:
- Mission-focused compacting can be improved by changing summarizer prompt schema and summary persistence semantics.

### 2.5 MCP tools exist
- MCP client creation and tool registration exists in [internal/llm/agent/mcp-tools.go](internal/llm/agent/mcp-tools.go).
- MCP server schema is in `Config.MCPServers` at [internal/config/config.go](internal/config/config.go).

Implication:
- Python and data-source connectors can be integrated either as MCP tools or native tools.

## 3) Gap Matrix

### Requirement A: Ollama-only + full runtime controls
Current status:
- Partial.
- Local endpoint path exists, but explicit Ollama controls (temperature, min_p, top_k, num_ctx, repeat_penalty, etc.) are not fully wired through request construction.

Needed:
- Add runtime generation options to config schema.
- Map options through provider request objects.
- Add validation and defaults by model/provider compatibility.

### Requirement B: SQLite indexing for local/network sources + RAG
Current status:
- Missing for indexing/retrieval.
- SQLite currently persists chat/session artifacts only.

Needed:
- New tables for sources, documents/chunks, embeddings metadata, ingestion jobs, retrieval logs.
- New SQLC queries and services.
- Indexing and retrieval tools plus orchestration hooks.
- First-class connectors for all target network sources (web, SQL, object storage).

### Requirement C: Extensibility for skills/tools/MCP/Python scripts
Current status:
- Partial.
- MCP is present; generic tool framework exists.

Needed:
- Standardized config blocks for tool/plugin registration.
- Python tool execution contract (timeouts, env, I/O schema, permission policy).
- Discovery and health-check commands.
- Combined enforcement model: allowlist-by-path plus interactive permission checks.

### Requirement D: Efficient compacting and focused flow
Current status:
- Partial.
- Compaction exists but summary objective is broad.

Needed:
- Structured summary format with mission-state fields.
- Compacting trigger policy and summary quality checks.
- Better handoff context extraction for next turn.

## 4) Recommended Phased Plan

### Phase 0: Lock architecture and constraints
Deliverables:
- Decision record documenting Ollama endpoint mode(s), embedding strategy, RAG schema boundaries, and tool-security posture.

Target files:
- Create architecture note in docs folder.
- Cross-reference [internal/config/config.go](internal/config/config.go), [internal/llm/provider/provider.go](internal/llm/provider/provider.go), [internal/db/connect.go](internal/db/connect.go).

Acceptance:
- No ambiguous runtime ownership or schema ownership remains.

### Phase 1: Ollama-first runtime controls
Deliverables:
- Config additions for generation parameters and provider-specific compatibility.
- Provider request plumbing for local/Ollama compatible payloads.
- Validation and sane defaults.

Likely touchpoints:
- [internal/config/config.go](internal/config/config.go)
- [internal/llm/agent/agent.go](internal/llm/agent/agent.go)
- [internal/llm/provider/openai.go](internal/llm/provider/openai.go)
- [internal/llm/provider/provider.go](internal/llm/provider/provider.go)
- [internal/llm/models/local.go](internal/llm/models/local.go)

Acceptance:
- User can set required sampling/runtime fields and verify they are applied end-to-end.

### Phase 2: SQLite indexing schema + ingestion pipeline
Deliverables:
- Migrations for source registry, document/chunk store, retrieval audit tables.
- SQLC query files and generated code.
- Ingestion service for local directories and selected network sources.

Likely touchpoints:
- [internal/db/migrations](internal/db/migrations)
- [internal/db/sql](internal/db/sql)
- [sqlc.yaml](sqlc.yaml)
- New service package under internal for indexing/RAG

Acceptance:
- Ingested data is queryable and traceable by source and job ID.

### Phase 3: Retrieval tools + RAG orchestration
Deliverables:
- Tools for index, search, and retrieve.
- Integration path from user query to retrieval augmentation.
- Optional MCP-backed connectors.
- Hybrid retrieval path (vector plus lexical) from first release.
- Query planner or reranker path that prioritizes retrieval quality and latency balance.

Likely touchpoints:
- [internal/llm/tools](internal/llm/tools)
- [internal/llm/agent/agent.go](internal/llm/agent/agent.go)
- [internal/llm/agent/mcp-tools.go](internal/llm/agent/mcp-tools.go)

Acceptance:
- Query can include retrieved context with source attribution.
- Retrieval path includes measurable performance targets and benchmark reporting.

### Phase 4: Extensibility framework (skills/tools/MCP/Python)
Deliverables:
- Config schema for custom tool registration.
- Python-script tool runner contract and permission controls.
- Discovery command and diagnostics.

Likely touchpoints:
- [internal/config/config.go](internal/config/config.go)
- [internal/permission/permission.go](internal/permission/permission.go)
- [internal/llm/tools](internal/llm/tools)

Acceptance:
- New tool integration requires only config + script, with predictable security behavior.

### Phase 5: Compacting and mission-focused summarization
Deliverables:
- Structured summarizer prompt format.
- Summary quality policy and compact trigger controls.
- Optional summary schema stored in DB for stable handoff.

Likely touchpoints:
- [internal/llm/prompt/summarizer.go](internal/llm/prompt/summarizer.go)
- [internal/llm/agent/agent.go](internal/llm/agent/agent.go)
- [internal/tui/tui.go](internal/tui/tui.go)
- [internal/session/session.go](internal/session/session.go)

Acceptance:
- Summaries retain decisions, pending tasks, risks, and immediate next action with low verbosity.

## 5) Execution Protocol For Any Agent

Use this exact flow for each phase:
1. Re-state phase objective and acceptance tests.
2. Inspect target files and confirm no conflicting in-flight changes.
3. Implement smallest coherent vertical slice.
4. Run validations (build/tests/migrations/lint as relevant).
5. Report:
- files changed
- behavior impact
- known risks
- exact next step

## 6) Validation Checklist

Minimum checks after each meaningful change:
- `go test ./...`
- `go test ./internal/llm/... ./internal/db/... ./internal/config/...`
- Migration up path verifies on a clean db file.
- Runtime config parses cleanly with invalid values rejected clearly.

For Ollama path:
- Validate model listing via `LOCAL_ENDPOINT` behavior in [internal/llm/models/local.go](internal/llm/models/local.go).
- Validate generation params are present in provider payload construction in [internal/llm/provider/openai.go](internal/llm/provider/openai.go).

For RAG path:
- Validate ingestion idempotency, chunk counts, source lineage, and retrieval relevance logging.
- Validate hybrid retrieval latency and quality against agreed benchmarks.

## 7) Example Prompts For The Custom Agent

- "Audit the current local provider flow and produce Phase 1 implementation tasks with file-level diffs." 
- "Implement Phase 1 only: add Ollama runtime controls to config and provider request wiring, then run targeted tests." 
- "Design and implement Phase 2 migration + SQLC query set for indexing sources and chunks." 
- "Refactor summarizer prompt and compacting logic to output mission-focused structured summaries." 

## 8) Open Decisions To Resolve Early

- Performance SLO targets for hybrid retrieval:
  - target p95 latency for top-k retrieval under expected corpus sizes
  - minimum quality threshold (for example NDCG@k or Recall@k baseline)
- Embedding model lifecycle policy:
  - re-embedding trigger rules when model version changes
  - background migration strategy and backward compatibility window
- Multi-source freshness policy:
  - ingestion cadence and conflict resolution when sources disagree

## 9) Non-Goals (to avoid scope drift)

- Do not optimize for cloud-provider parity in early phases.
- Do not build every connector at once.
- Do not redesign unrelated TUI or styling behavior unless needed for mission-critical controls.

## 10) Handoff Standard

When handing off to another agent, include:
- Current phase and acceptance status.
- Blocking decisions still unresolved.
- Exact files and tests touched in last change.
- Next smallest high-confidence task.

This keeps execution focused, efficient, and reproducible.
