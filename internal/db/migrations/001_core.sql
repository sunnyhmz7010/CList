CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS storage_profiles (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL CHECK (type IN ('local', 'telegram_official', 'telegram_streaming')),
  name TEXT NOT NULL,
  encrypted_config BLOB NOT NULL DEFAULT X'',
  enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
  is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_profiles_default
  ON storage_profiles(is_default) WHERE is_default = 1;

CREATE TABLE IF NOT EXISTS guest_vaults (
  id TEXT PRIMARY KEY,
  key_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  revoked_at TEXT
);

CREATE TABLE IF NOT EXISTS folders (
  id TEXT PRIMARY KEY,
  parent_id TEXT REFERENCES folders(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  owner_vault_id TEXT REFERENCES guest_vaults(id) ON DELETE SET NULL,
  gallery_visibility TEXT NOT NULL DEFAULT 'inherit'
    CHECK (gallery_visibility IN ('inherit', 'visible', 'hidden')),
  state TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'trashed', 'purged')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_folders_active_name
  ON folders(COALESCE(parent_id, ''), COALESCE(owner_vault_id, ''), name)
  WHERE state = 'active';
CREATE INDEX IF NOT EXISTS idx_folders_parent_id ON folders(parent_id, id);
CREATE INDEX IF NOT EXISTS idx_folders_owner_vault_id ON folders(owner_vault_id, id);

CREATE TABLE IF NOT EXISTS files (
  public_id TEXT PRIMARY KEY,
  folder_id TEXT REFERENCES folders(id) ON DELETE SET NULL,
  owner_vault_id TEXT REFERENCES guest_vaults(id) ON DELETE SET NULL,
  storage_profile_id TEXT NOT NULL REFERENCES storage_profiles(id) ON DELETE RESTRICT,
  storage_key TEXT,
  file_name TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size INTEGER NOT NULL CHECK (size >= 0),
  sha256 TEXT NOT NULL,
  gallery_visibility TEXT NOT NULL DEFAULT 'inherit'
    CHECK (gallery_visibility IN ('inherit', 'visible', 'hidden')),
  state TEXT NOT NULL CHECK (state IN ('uploading', 'active', 'trashed', 'purged')),
  purged_at TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_files_state_created ON files(state, created_at, public_id);
CREATE INDEX IF NOT EXISTS idx_files_folder_id ON files(folder_id, public_id);
CREATE INDEX IF NOT EXISTS idx_files_owner_vault_id ON files(owner_vault_id, public_id);
CREATE INDEX IF NOT EXISTS idx_files_storage_profile_id ON files(storage_profile_id, public_id);

CREATE TABLE IF NOT EXISTS file_secrets (
  file_id TEXT PRIMARY KEY REFERENCES files(public_id) ON DELETE CASCADE,
  password_hash BLOB NOT NULL,
  salt BLOB NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trash_batches (
  id TEXT PRIMARY KEY,
  root_type TEXT NOT NULL CHECK (root_type IN ('file', 'folder')),
  root_id TEXT NOT NULL,
  actor_kind TEXT NOT NULL,
  actor_id TEXT,
  deleted_at TEXT NOT NULL,
  restored_at TEXT
);

CREATE TABLE IF NOT EXISTS trash_items (
  id TEXT PRIMARY KEY,
  batch_id TEXT NOT NULL REFERENCES trash_batches(id) ON DELETE CASCADE,
  item_type TEXT NOT NULL CHECK (item_type IN ('file', 'folder')),
  item_id TEXT NOT NULL,
  original_parent_id TEXT,
  original_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(batch_id, item_type, item_id)
);

CREATE INDEX IF NOT EXISTS idx_trash_items_batch_id ON trash_items(batch_id, id);

CREATE TABLE IF NOT EXISTS telegram_messages (
  chat_id TEXT NOT NULL,
  message_id INTEGER NOT NULL,
  storage_profile_id TEXT NOT NULL REFERENCES storage_profiles(id) ON DELETE RESTRICT,
  file_public_id TEXT REFERENCES files(public_id) ON DELETE SET NULL,
  file_id TEXT NOT NULL,
  file_unique_id TEXT NOT NULL,
  file_size INTEGER NOT NULL CHECK (file_size >= 0),
  created_at TEXT NOT NULL,
  PRIMARY KEY(chat_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_telegram_messages_file_public_id
  ON telegram_messages(file_public_id);

INSERT OR IGNORE INTO storage_profiles (
  id, type, name, encrypted_config, enabled, is_default, created_at, updated_at
) VALUES (
  'local-default', 'local', '本地存储', X'', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
);
