package audit

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
)

type Service struct{ db *sql.DB }

func NewService(database *sql.DB) *Service { return &Service{db: database} }

func (s *Service) Record(ctx context.Context, action, targetID string, actor auth.Actor, requestID, result string) error {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs(id,action,target_public_id,actor_kind,actor_id,request_id,created_at,result) VALUES(?,?,?,?,?,?,?,?)`, hex.EncodeToString(id), action, nullable(targetID), actor.Kind, nullable(actor.ID), nullable(requestID), time.Now().UTC().Format(time.RFC3339Nano), result)
	return err
}

func Redact(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "cookie") {
			result[key] = "[REDACTED]"
		} else {
			result[key] = value
		}
	}
	return result
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
