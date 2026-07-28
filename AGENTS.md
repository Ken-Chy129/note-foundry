# AGENTS.md

## Project mission

Build a single-owner personal learning workspace. The first useful product is a dependable knowledge-management system; AI features are later layers, not prerequisites for v0.1.

## Required context

Before planning or changing code, read these files in order:

1. `CONTEXT.md` — canonical domain vocabulary. Use these terms consistently and keep implementation details out of this file.
2. `docs/product-scope.md` — release boundaries and v0.1 acceptance scenario.
3. `docs/architecture.md` — agreed system shape and module boundaries.
4. Relevant files in `docs/adr/` — decisions that should not be silently reversed.

If a proposed implementation conflicts with these documents, surface the conflict before coding. Update documentation when a product or architecture decision genuinely changes.

## Current phase

- The repository contains design context only.
- The next development target is v0.1, the knowledge-management core.
- Learning Sources belong to v0.2.
- RAG, Agent execution, memory, Learning Topics, RSS, and news ingestion belong to v0.3.
- Do not introduce deferred concepts into the v0.1 data model or UI unless they are required for a documented compatibility seam.

## Technical baseline

- Next.js with TypeScript for the web application.
- Go modular monolith for the API and worker roles.
- PostgreSQL as runtime system of record.
- REST described by OpenAPI; SSE may be added only for a concrete need.
- PostgreSQL `tsvector` plus `pg_trgm` for search.
- Docker Compose and Caddy for the first deployment.
- PostgreSQL-backed durable Jobs; no Redis, Kafka, or external message broker in v0.1.

## Domain invariants

- A Learning Note is the primary page-sized knowledge unit and stores canonical Markdown.
- Note identity is stable across title, slug, path, and Knowledge Space changes.
- Knowledge Spaces are flat top-level containers; only directories inside them nest.
- Note visibility follows its Knowledge Space.
- Editing a public note does not change its Published Content until explicit publish.
- Only the configured GitHub owner can write; public readers are anonymous and read-only.
- Tags are global and flat.
- Backlinks are derived from stable Note Links.
- Attachments inherit the visibility of their owning content.
- Trash has no automatic expiry; permanent deletion is explicit.
- Search indexes and other projections must be rebuildable from canonical data.

## Working expectations

- Prefer incremental, reviewable changes tied to the v0.1 acceptance scenario.
- Organize Go code by business capability as described in `docs/architecture.md`.
- Add tests for domain rules and persistence behavior as functionality is introduced.
- Keep generated files and secrets out of Git.
- Avoid premature multi-user permissions, microservices, semantic search, block-level knowledge models, and generic workflow engines.

## Definition of done for v0.1

The release is not complete until the full Hermes Agent study scenario in `docs/product-scope.md` has been exercised on a deployed instance, including a real encrypted backup and restore.

