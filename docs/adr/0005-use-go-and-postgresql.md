# Use Go and PostgreSQL

The modular monolith backend is implemented in Go and uses PostgreSQL as its primary database. Go matches the owner's existing expertise, supports a small operational footprint, and leaves a direct path to Go-native Agent tooling later; PostgreSQL is chosen over MySQL for its composable weighted text search, extension model, and the option to add vector and hybrid retrieval without introducing a separate database, despite MySQL also being sufficient for the first-version knowledge-management workload.
