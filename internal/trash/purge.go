package trash

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/storage"
)

type PurgeService struct {
	db       *sql.DB
	registry *storage.Registry
}

func NewPurgeService(database *sql.DB, registry *storage.Registry) *PurgeService {
	return &PurgeService{db: database, registry: registry}
}

func (s *PurgeService) PurgeFile(ctx context.Context, publicID string, actor auth.Actor) error {
	var owner, key sql.NullString
	var profileID, state string
	err := s.db.QueryRowContext(ctx, "SELECT owner_vault_id,storage_profile_id,storage_key,state FROM files WHERE public_id=?", publicID).Scan(&owner, &profileID, &key, &state)
	if err != nil {
		return err
	}
	if state == string(repository.FileStatePurged) {
		return nil
	}
	if state != string(repository.FileStateTrashed) || !canManage(actor, owner.String) {
		return files.ErrForbidden
	}
	backend, err := s.registry.Resolve(profileID)
	if err != nil {
		return s.recordError(ctx, publicID, err)
	}
	if key.String != "" {
		err = backend.Delete(ctx, key.String)
		if err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
			return s.recordError(ctx, publicID, err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE files SET state='purged',storage_key=NULL,owner_vault_id=NULL,folder_id=NULL,purged_at=?,last_error=NULL,updated_at=? WHERE public_id=?`, now(), now(), publicID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM file_secrets WHERE file_id=?", publicID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE telegram_messages SET file_public_id=NULL WHERE file_public_id=?", publicID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PurgeService) PurgeBatch(ctx context.Context, batchID string, actor auth.Actor) error {
	rows, err := s.db.QueryContext(ctx, "SELECT item_id FROM trash_items WHERE batch_id=? AND item_type='file' ORDER BY created_at,id", batchID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.PurgeFile(ctx, id, actor); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, "UPDATE folders SET state='purged',owner_vault_id=NULL,parent_id=NULL,updated_at=? WHERE id IN (SELECT item_id FROM trash_items WHERE batch_id=? AND item_type='folder')", time.Now().UTC().Format(time.RFC3339Nano), batchID)
	return err
}

func (s *PurgeService) recordError(ctx context.Context, publicID string, operationErr error) error {
	_, err := s.db.ExecContext(ctx, "UPDATE files SET last_error=?,updated_at=? WHERE public_id=?", operationErr.Error(), now(), publicID)
	if err != nil {
		return err
	}
	return operationErr
}
