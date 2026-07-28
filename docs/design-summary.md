# NoteFoundry — Agreed Design

## One-sentence definition

A single-owner, server-hosted personal knowledge workspace whose first job is to let the owner write, organize, search, publish, and safely retain reusable learning notes; Source collection arrives next, and Agent/RAG capabilities build on top only after the knowledge core is dependable.

## Product model

- **Knowledge Space**: flat top-level container such as `AI Agent`, `Java`, or `Database`; may be public or private.
- **Directory**: nested structure inside a Knowledge Space.
- **Learning Note**: page-sized, Markdown-based, stable-identity knowledge unit.
- **Note Draft / Published Content / Note Revision**: separate working, public, and recoverable states.
- **Tag**: flat, global, cross-space classification.
- **Note Link**: stable forward reference with automatically derived backlinks.
- **Learning Source**: private external input such as a webpage or PDF; introduced in v0.2.
- **Source Inbox**: unorganized Source view; introduced in v0.2.

## Release plan

### v0.1 — Use it to learn Hermes Agent

- GitHub owner login.
- Knowledge Spaces and directories.
- Markdown editor and preview.
- Autosave, conflict detection, drafts, publishing, and lightweight revisions.
- Tags, stable links, and backlinks.
- PostgreSQL full-text search.
- Public knowledge-base pages.
- Attachments, Trash, and encrypted automatic backups.
- Docker Compose deployment through Caddy.

### v0.2 — Capture learning material

- URL/PDF/manual Sources.
- Source Inbox and capture remarks.
- Async extraction with PostgreSQL Jobs.
- Deduplication and note-to-source citations.

### v0.3 — Learn Agent technology through the product

- RAG and semantic retrieval.
- Reviewable Agent note proposals.
- Memory and context management.
- Learning Topics.
- RSS, news, and automated collection.

## Technical architecture

```text
Caddy
├── Next.js + TypeScript
└── Go REST/OpenAPI modular monolith
    ├── API role
    └── Worker role

PostgreSQL
├── canonical Markdown
├── metadata and relationships
├── revisions and jobs
└── tsvector + pg_trgm search

Local managed storage
└── attachments

Encrypted S3-compatible storage
└── automatic offsite backups
```

## Core design principles

1. Notes, not AI output, are the primary knowledge unit.
2. Markdown is canonical, but PostgreSQL is the runtime system of record.
3. Knowledge organization is tree-first and relationship-assisted.
4. Public/private visibility belongs to Knowledge Spaces, not a general ACL system.
5. Note identity is independent of title, slug, and path.
6. Public editing uses drafts and explicit publishing.
7. Slow work is asynchronous and durable, without adding a message broker.
8. Search remains in PostgreSQL until observed scale or quality demands more.
9. Agent capabilities must propose changes before modifying confirmed knowledge.
10. Each release must already be useful without waiting for the AI roadmap.

## v0.1 completion test

Deploy the product, create an `AI Agent/Hermes Agent` hierarchy, write and publish five genuine Hermes Agent design notes, connect them with stable links and backlinks, find them through Chinese and English search, verify public/private isolation, restore a deleted note, and perform a real backup restore. Only then is v0.1 complete.
