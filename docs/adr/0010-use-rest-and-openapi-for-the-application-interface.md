# Use REST and OpenAPI for the application interface

The Next.js frontend communicates with the Go modular monolith through a REST interface described by OpenAPI. Resource endpoints cover queries and ordinary mutations, while explicit command endpoints represent actions such as publish, restore, move, and retry extraction; server-sent events may be added for concrete streaming needs, but GraphQL and subscription infrastructure are excluded from the first version.
