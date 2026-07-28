# Deploy the system with Docker Compose and Caddy

The personal server deployment uses Docker Compose with Caddy as the HTTPS reverse proxy, a Next.js web container, Go API and worker roles built from the same backend codebase, PostgreSQL, and managed persistent volumes for database data, attachments, and Caddy state. Redis, external message brokers, and separate search services are excluded; Caddy routes API and protected attachment requests to Go and all other requests to Next.js.
