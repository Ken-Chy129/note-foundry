# Store runtime attachments on local managed storage

Binary attachments are stored in a managed persistent directory on the application server while PostgreSQL stores ownership, visibility-related metadata, immutable storage keys, media types, sizes, and checksums. The attachment interface must remain storage-backend-neutral so object storage can replace the local directory later; encrypted offsite backups include both the database and attachment directory, while the S3-compatible backup destination is not a runtime dependency.

Canonical Markdown references a managed file with an `attachment:<uuid>` destination, for example `![Agent Loop](attachment:11111111-1111-4111-8111-111111111111)`. Upload alone never makes a file public: publishing a Learning Note atomically marks only the Attachments referenced by that Published Content as publicly eligible, and every anonymous download rechecks the owning Note and Knowledge Space visibility.
