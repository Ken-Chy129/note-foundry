# Store runtime attachments on local managed storage

Binary attachments are stored in a managed persistent directory on the application server while PostgreSQL stores ownership, visibility-related metadata, immutable storage keys, media types, sizes, and checksums. The attachment interface must remain storage-backend-neutral so object storage can replace the local directory later; encrypted offsite backups include both the database and attachment directory, while the S3-compatible backup destination is not a runtime dependency.
