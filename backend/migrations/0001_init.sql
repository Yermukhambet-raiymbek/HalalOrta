-- 0001_init: bookkeeping only.
-- Creates the table the migration runner uses to record applied versions.
-- Product tables arrive in later migrations.
CREATE TABLE schema_migrations (
    version    text        PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
