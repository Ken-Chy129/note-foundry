# Use PostgreSQL-backed background jobs

Slow or failure-prone work runs asynchronously through durable Jobs stored in PostgreSQL. v0.1 first uses this mechanism for scheduled backups; v0.2 adds web extraction and PDF parsing after a Learning Source has been created. The modular monolith executes Jobs through a worker from the same codebase, avoiding an external message broker while providing durable retries and a shared foundation for later ingestion, indexing, and Agent tasks.
