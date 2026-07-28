# Encrypted offsite backups are required

The first version automatically creates a consistent backup of PostgreSQL data and managed attachments, encrypts it before upload, and stores it in an S3-compatible location independent of the application server. Daily and weekly retention plus a supported restore command are part of the design, because a database-backed personal knowledge system must survive server, volume, deployment, and migration failures rather than relying on the application host as its only copy.
