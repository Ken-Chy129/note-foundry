# NoteFoundry — Product Scope

## Product position

The product is a single-owner learning workspace for writing, organizing, publishing, and safely retaining personal learning notes. It is not a general collaboration suite, a blog, an AI chat product, or a web archive.

The primary knowledge unit is the **Learning Note**. External material is an input to learning and remains distinct from user-approved knowledge.

## Release roadmap

### v0.1 — Knowledge-management core

- GitHub OAuth restricted to one configured Knowledge Owner.
- Flat top-level Knowledge Spaces with nested directories.
- Public or private visibility at the Knowledge Space level.
- Markdown editing with preview, code blocks, Mermaid, formulas, tables, and attachments.
- Frequent autosave with optimistic concurrency checks.
- Draft and publish flow for public notes.
- Lightweight Note Revisions and restore.
- Flat global Tags with rename and merge.
- Stable Note identities, forward links, and automatically derived backlinks.
- PostgreSQL full-text search using weighted `tsvector` plus `pg_trgm` fallback.
- Public knowledge-base navigation and anonymous public search.
- Trash with manual permanent deletion.
- Local managed attachment storage with inherited visibility.
- Encrypted automatic offsite backups to S3-compatible storage and a supported restore command.
- Docker Compose deployment behind Caddy.

### v0.2 — Learning Sources

- URL, PDF, and manual source capture.
- Source Inbox and Capture Notes.
- One primary Knowledge Space per organized source, displayed separately from notes.
- Asynchronous web extraction and PDF parsing through PostgreSQL-backed Jobs.
- URL normalization and file-hash deduplication.
- Private source management with public citation metadata only.
- Note-to-source references.

### v0.3 — AI learning capabilities

- Scoped RAG and semantic retrieval.
- Agent-created Note Drafts and reviewable modification proposals.
- Memory and context-management experiments.
- Learning Topics with goals, questions, progress, and source plans.
- RSS, news, browser, and scheduled collection channels.

## Explicitly excluded from v0.1

- Multiple users, teams, comments, or collaborative editing.
- Per-note ACLs, password links, or unlisted visibility.
- Learning Topics and project-management workflows.
- Learning Sources and automated collection.
- Block-level knowledge objects or block references.
- Knowledge-graph visualization.
- Semantic search, embeddings, RAG, and Agent execution.
- GraphQL, microservices, Redis, Kafka, and external search engines.
- Full rich-text editing that cannot round-trip through Markdown.

## v0.1 acceptance scenario

v0.1 is complete only when the Knowledge Owner can use the deployed system for a real Hermes Agent study:

1. Sign in through GitHub on the configured domain.
2. Create a public `AI Agent` Knowledge Space and a nested `Hermes Agent` directory.
3. Write and publish at least five real notes covering architecture, Agent Loop, memory, context management, and tools/skills.
4. Use code blocks, Mermaid, images, tags, and internal Note Links.
5. Follow a forward link and observe the automatically derived backlink.
6. Edit a public note without exposing incomplete draft content.
7. Find notes through both Chinese and English search terms.
8. Confirm anonymous readers can access only published content in public spaces.
9. Restore a deleted note from Trash without changing its stable identity.
10. Verify encrypted automatic backup upload and perform at least one real restore.
11. Read comfortably on mobile and edit comfortably on desktop.
