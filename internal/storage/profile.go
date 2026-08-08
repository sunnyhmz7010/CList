package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"time"

	clistcrypto "github.com/sunnyhmz7010/CList/internal/crypto"
)

var ErrInvalidProfile = errors.New("invalid storage profile")

type ProfileInput struct {
	Type, Name string
	Config     map[string]string
	Enabled    bool
}
type Profile struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Name         string       `json:"name"`
	Enabled      bool         `json:"enabled"`
	IsDefault    bool         `json:"isDefault"`
	Capabilities Capabilities `json:"capabilities"`
}
type ProfileService struct {
	db       *sql.DB
	key      []byte
	registry *Registry
}

func NewProfileService(database *sql.DB, key []byte, registry *Registry) *ProfileService {
	return &ProfileService{db: database, key: key, registry: registry}
}

func (s *ProfileService) Create(ctx context.Context, input ProfileInput) (Profile, error) {
	if err := validateProfile(input); err != nil {
		return Profile{}, err
	}
	backend, err := s.registry.Build(input.Type, input.Config)
	if err != nil {
		return Profile{}, err
	}
	raw, err := json.Marshal(input.Config)
	if err != nil {
		return Profile{}, err
	}
	encrypted, err := clistcrypto.EncryptConfig(s.key, raw)
	if err != nil {
		return Profile{}, err
	}
	id, err := profileID()
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO storage_profiles(id,type,name,encrypted_config,enabled,is_default,created_at,updated_at)
VALUES(?,?,?,?,?,0,?,?)`, id, input.Type, input.Name, encrypted, boolInt(input.Enabled), now, now)
	if err != nil {
		return Profile{}, err
	}
	s.registry.Register(id, backend)
	return s.get(ctx, id)
}

// LoadEnabled 在进程启动时恢复已启用的存储档案。
func (s *ProfileService) LoadEnabled(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id,type,encrypted_config FROM storage_profiles WHERE enabled=1 ORDER BY created_at,id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, kind string
		var encrypted []byte
		if err := rows.Scan(&id, &kind, &encrypted); err != nil {
			return err
		}
		config, err := s.decryptConfig(id, encrypted)
		if err != nil {
			return err
		}
		if err := s.registry.RegisterConfig(id, kind, config); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *ProfileService) List(ctx context.Context) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,type,name,enabled,is_default FROM storage_profiles ORDER BY created_at,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var profiles []Profile
	for rows.Next() {
		var p Profile
		var enabled, def int
		if err := rows.Scan(&p.ID, &p.Type, &p.Name, &enabled, &def); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		p.IsDefault = def == 1
		if backend, err := s.registry.Resolve(p.ID); err == nil {
			p.Capabilities = backend.Capabilities()
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (s *ProfileService) SetDefault(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE storage_profiles SET is_default=0 WHERE is_default=1"); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE storage_profiles SET is_default=1,updated_at=? WHERE id=? AND enabled=1", time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrInvalidProfile
	}
	return tx.Commit()
}

func (s *ProfileService) Config(ctx context.Context, id string) (map[string]string, error) {
	var encrypted []byte
	if err := s.db.QueryRowContext(ctx, "SELECT encrypted_config FROM storage_profiles WHERE id=?", id).Scan(&encrypted); err != nil {
		return nil, err
	}
	return s.decryptConfig(id, encrypted)
}

func (s *ProfileService) decryptConfig(id string, encrypted []byte) (map[string]string, error) {
	if len(encrypted) == 0 && id == "local-default" {
		return map[string]string{"root": "/data/files"}, nil
	}
	raw, err := clistcrypto.DecryptConfig(s.key, encrypted)
	if err != nil {
		return nil, err
	}
	var config map[string]string
	if err = json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *ProfileService) get(ctx context.Context, id string) (Profile, error) {
	var p Profile
	var enabled, def int
	err := s.db.QueryRowContext(ctx, "SELECT id,type,name,enabled,is_default FROM storage_profiles WHERE id=?", id).Scan(&p.ID, &p.Type, &p.Name, &enabled, &def)
	p.Enabled = enabled == 1
	p.IsDefault = def == 1
	if b, e := s.registry.Resolve(id); e == nil {
		p.Capabilities = b.Capabilities()
	}
	return p, err
}

func validateProfile(input ProfileInput) error {
	if input.Name == "" {
		return ErrInvalidProfile
	}
	switch input.Type {
	case "local":
		root := input.Config["root"]
		if !filepath.IsAbs(root) {
			return ErrInvalidProfile
		}
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			return err
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return ErrInvalidProfile
		}
	case "telegram_official", "telegram_streaming":
		parsed, err := url.Parse(input.Config["base_url"])
		if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return ErrInvalidProfile
		}
		if input.Config["bot_token"] == "" || input.Config["channel_id"] == "" {
			return ErrInvalidProfile
		}
	default:
		return ErrInvalidProfile
	}
	return nil
}
func profileID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
