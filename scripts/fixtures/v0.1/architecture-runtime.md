# Architecture and Runtime

Hermes uses a small runtime that separates planning, tool execution, and durable state. The database remains the source of truth while workers claim durable jobs.

```mermaid
flowchart LR
  U[User intent] --> L[Agent loop]
  L --> T[Tool boundary]
  T --> W[Worker]
  W --> D[(Durable state)]
  D --> L
```

![Hermes runtime map](attachment:{{ATTACHMENT_ID}})

The diagram attachment is managed by NoteFoundry and becomes public only when this Published Content references it.

Return to the [Hermes Agent overview](note:{{OVERVIEW_ID}}).
