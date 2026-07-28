# Organize Go code by business capability

The Go backend is organized into cohesive business modules such as identity, knowledge organization, notes, sources, search, attachments, jobs, and backup rather than global handler, service, repository, and model directories. Each module owns its rules, application operations, persistence adapters, HTTP handlers, and tests, and other modules interact through explicit interfaces or published events instead of directly mutating its tables.
