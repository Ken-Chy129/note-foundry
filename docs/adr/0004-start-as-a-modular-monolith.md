# Start as a modular monolith

The first version is built as a modular monolith with one primary backend application, one database, and clearly separated modules for identity, knowledge organization, notes, sources, search, public reading, and background work. Collection automation, RAG, and Agent capabilities should initially join the same system as modules or workers and be extracted into independent services only when they develop concrete isolation or scaling requirements.
