CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  state TEXT NOT NULL
    CHECK (state IN ('queued', 'running', 'retry_wait', 'succeeded', 'failed', 'cleanup_pending', 'uncertain')),
  progress REAL NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 1),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  lease_until TEXT,
  next_attempt_at TEXT,
  last_error TEXT,
  payload BLOB,
  result BLOB,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_state_next_attempt
  ON jobs(state, next_attempt_at, created_at);
CREATE INDEX IF NOT EXISTS idx_jobs_lease_until ON jobs(lease_until);

CREATE TABLE IF NOT EXISTS upload_sessions (
  id TEXT PRIMARY KEY,
  owner_vault_id TEXT REFERENCES guest_vaults(id) ON DELETE SET NULL,
  storage_profile_id TEXT NOT NULL REFERENCES storage_profiles(id) ON DELETE RESTRICT,
  folder_id TEXT REFERENCES folders(id) ON DELETE SET NULL,
  file_public_id TEXT REFERENCES files(public_id) ON DELETE SET NULL,
  file_name TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  total_size INTEGER NOT NULL CHECK (total_size >= 0),
  chunk_size INTEGER NOT NULL CHECK (chunk_size > 0),
  total_chunks INTEGER NOT NULL CHECK (total_chunks > 0),
  sha256 TEXT NOT NULL,
  state TEXT NOT NULL
    CHECK (state IN ('uploading', 'completing', 'completed', 'aborted', 'failed')),
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  expires_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_upload_sessions_state_updated
  ON upload_sessions(state, updated_at);

CREATE TABLE IF NOT EXISTS upload_chunks (
  upload_id TEXT NOT NULL REFERENCES upload_sessions(id) ON DELETE CASCADE,
  chunk_index INTEGER NOT NULL CHECK (chunk_index >= 0),
  size INTEGER NOT NULL CHECK (size >= 0),
  sha256 TEXT NOT NULL,
  path TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(upload_id, chunk_index)
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  scope TEXT NOT NULL,
  key TEXT NOT NULL,
  operation TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('reserved', 'completed')),
  resource_id TEXT,
  response_status INTEGER,
  response_body BLOB,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  PRIMARY KEY(scope, key, operation)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at
  ON idempotency_keys(expires_at);
