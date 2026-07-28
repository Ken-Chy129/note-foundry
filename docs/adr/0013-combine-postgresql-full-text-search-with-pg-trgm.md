# Combine PostgreSQL full-text search with pg_trgm

First-version search remains inside PostgreSQL and combines weighted `tsvector`/GIN indexes for titles, tags, headings, body text, and technical tokens with `pg_trgm` indexes for Chinese character sequences, substring matching, and similarity fallback. Search documents are rebuildable projections of canonical Markdown and source text; external search engines and specialized Chinese tokenization services are deferred until observed search quality or scale justifies them.
