# Database is the runtime system of record

The application database is the sole runtime system of record for Learning Note Markdown, hierarchy, visibility, relationships, drafts, revisions, and Learning Sources. Markdown remains the canonical content format and must be fully importable and exportable, while binary attachments live in managed object or file storage; this avoids consistency problems between editable files and application metadata and ensures that all writes pass through revision and approval rules.
