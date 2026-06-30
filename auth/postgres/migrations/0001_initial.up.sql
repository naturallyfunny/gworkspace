CREATE TABLE IF NOT EXISTS "gworkspace_tokens" (
    "owner"         text                     NOT NULL PRIMARY KEY,
    "refresh_token" text                     NOT NULL,
    "created_at"    timestamp with time zone DEFAULT NOW()
);
