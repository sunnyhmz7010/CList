package trash

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
)

var ErrRestoreConflict = errors.New("trash restore conflict")

type Service struct{ db *sql.DB }

func NewService(database *sql.DB) *Service { return &Service{db: database} }

type Batch struct {
	ID        string `json:"id"`
	RootType  string `json:"root_type"`
	RootID    string `json:"root_id"`
	DeletedAt string `json:"deleted_at"`
	Items     []Item `json:"items"`
}

type Item struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	ItemID           string `json:"item_id"`
	OriginalParentID string `json:"original_parent_id,omitempty"`
	OriginalName     string `json:"original_name"`
}

func (s *Service) DeleteFile(ctx context.Context, publicID string, actor auth.Actor) (Batch, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Batch{}, err
	}
	defer tx.Rollback()
	var owner, parent sql.NullString
	var name, state string
	err = tx.QueryRowContext(ctx, "SELECT owner_vault_id,folder_id,file_name,state FROM files WHERE public_id=?", publicID).Scan(&owner, &parent, &name, &state)
	if err != nil {
		return Batch{}, err
	}
	if state != string(repository.FileStateActive) || !canManage(actor, owner.String) {
		return Batch{}, files.ErrForbidden
	}
	batch, err := s.createBatch(ctx, tx, "file", publicID, actor)
	if err != nil {
		return Batch{}, err
	}
	item, err := s.insertItem(ctx, tx, batch.ID, "file", publicID, parent.String, name)
	if err != nil {
		return Batch{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE files SET state='trashed',updated_at=? WHERE public_id=?", now(), publicID); err != nil {
		return Batch{}, err
	}
	if err := tx.Commit(); err != nil {
		return Batch{}, err
	}
	batch.Items = []Item{item}
	return batch, nil
}

func (s *Service) DeleteFolder(ctx context.Context, folderID string, actor auth.Actor) (Batch, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Batch{}, err
	}
	defer tx.Rollback()
	var owner sql.NullString
	var state string
	if err := tx.QueryRowContext(ctx, "SELECT owner_vault_id,state FROM folders WHERE id=?", folderID).Scan(&owner, &state); err != nil {
		return Batch{}, err
	}
	if state != string(repository.FolderStateActive) || !canManage(actor, owner.String) {
		return Batch{}, files.ErrForbidden
	}
	batch, err := s.createBatch(ctx, tx, "folder", folderID, actor)
	if err != nil {
		return Batch{}, err
	}
	cte := `WITH RECURSIVE descendants(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN descendants d ON f.parent_id=d.id WHERE f.state='active') `
	var snapshots []Item
	folderRows, err := tx.QueryContext(ctx, cte+`SELECT f.id,f.parent_id,f.name FROM folders f JOIN descendants d ON d.id=f.id ORDER BY f.created_at,f.id`, folderID)
	if err != nil {
		return Batch{}, err
	}
	for folderRows.Next() {
		var id, name string
		var parent sql.NullString
		if err := folderRows.Scan(&id, &parent, &name); err != nil {
			folderRows.Close()
			return Batch{}, err
		}
		snapshots = append(snapshots, Item{Type: "folder", ItemID: id, OriginalParentID: parent.String, OriginalName: name})
	}
	folderRows.Close()
	fileRows, err := tx.QueryContext(ctx, cte+`SELECT f.public_id,f.folder_id,f.file_name FROM files f JOIN descendants d ON d.id=f.folder_id WHERE f.state='active' ORDER BY f.created_at,f.public_id`, folderID)
	if err != nil {
		return Batch{}, err
	}
	for fileRows.Next() {
		var id, name string
		var parent sql.NullString
		if err := fileRows.Scan(&id, &parent, &name); err != nil {
			fileRows.Close()
			return Batch{}, err
		}
		snapshots = append(snapshots, Item{Type: "file", ItemID: id, OriginalParentID: parent.String, OriginalName: name})
	}
	fileRows.Close()
	for _, snapshot := range snapshots {
		item, err := s.insertItem(ctx, tx, batch.ID, snapshot.Type, snapshot.ItemID, snapshot.OriginalParentID, snapshot.OriginalName)
		if err != nil {
			return Batch{}, err
		}
		batch.Items = append(batch.Items, item)
	}
	if _, err := tx.ExecContext(ctx, cte+`UPDATE files SET state='trashed',updated_at=? WHERE folder_id IN (SELECT id FROM descendants) AND state='active'`, folderID, now()); err != nil {
		return Batch{}, err
	}
	if _, err := tx.ExecContext(ctx, cte+`UPDATE folders SET state='trashed',updated_at=? WHERE id IN (SELECT id FROM descendants)`, folderID, now()); err != nil {
		return Batch{}, err
	}
	if err := tx.Commit(); err != nil {
		return Batch{}, err
	}
	return batch, nil
}

func (s *Service) List(ctx context.Context, actor auth.Actor) ([]Batch, error) {
	query := "SELECT id,root_type,root_id,deleted_at FROM trash_batches WHERE restored_at IS NULL"
	args := []any{}
	if actor.Kind != auth.ActorAdmin {
		query += " AND actor_kind=? AND actor_id=?"
		args = append(args, actor.Kind, actor.ID)
	}
	query += " ORDER BY deleted_at DESC,id"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var batches []Batch
	for rows.Next() {
		var batch Batch
		if err := rows.Scan(&batch.ID, &batch.RootType, &batch.RootID, &batch.DeletedAt); err != nil {
			return nil, err
		}
		items, err := s.listItems(ctx, batch.ID)
		if err != nil {
			return nil, err
		}
		batch.Items = items
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func (s *Service) RestoreBatch(ctx context.Context, batchID string, actor auth.Actor) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var actorKind string
	var actorID sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT actor_kind,actor_id FROM trash_batches WHERE id=? AND restored_at IS NULL", batchID).Scan(&actorKind, &actorID); err != nil {
		return err
	}
	if actor.Kind != auth.ActorAdmin && (actorKind != string(actor.Kind) || actorID.String != actor.ID) {
		return files.ErrForbidden
	}
	items, err := s.listItemsTx(ctx, tx, batchID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if conflict, err := s.restoreConflict(ctx, tx, item); err != nil {
			return err
		} else if conflict {
			return ErrRestoreConflict
		}
	}
	for _, item := range items {
		table, key := "files", "public_id"
		if item.Type == "folder" {
			table, key = "folders", "id"
		}
		if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET state='active',updated_at=? WHERE "+key+"=? AND state='trashed'", now(), item.ItemID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE trash_batches SET restored_at=? WHERE id=?", now(), batchID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) createBatch(ctx context.Context, tx *sql.Tx, rootType, rootID string, actor auth.Actor) (Batch, error) {
	id, err := randomID()
	if err != nil {
		return Batch{}, err
	}
	deletedAt := now()
	_, err = tx.ExecContext(ctx, "INSERT INTO trash_batches(id,root_type,root_id,actor_kind,actor_id,deleted_at) VALUES(?,?,?,?,?,?)", id, rootType, rootID, actor.Kind, nullable(actor.ID), deletedAt)
	return Batch{ID: id, RootType: rootType, RootID: rootID, DeletedAt: deletedAt}, err
}

func (s *Service) insertItem(ctx context.Context, tx *sql.Tx, batchID, typ, itemID, parentID, name string) (Item, error) {
	id, err := randomID()
	if err != nil {
		return Item{}, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO trash_items(id,batch_id,item_type,item_id,original_parent_id,original_name,created_at) VALUES(?,?,?,?,?,?,?)", id, batchID, typ, itemID, nullable(parentID), name, now())
	return Item{ID: id, Type: typ, ItemID: itemID, OriginalParentID: parentID, OriginalName: name}, err
}

func (s *Service) listItems(ctx context.Context, batchID string) ([]Item, error) {
	return s.listItemsTx(ctx, s.db, batchID)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Service) listItemsTx(ctx context.Context, query queryer, batchID string) ([]Item, error) {
	rows, err := query.QueryContext(ctx, "SELECT id,item_type,item_id,original_parent_id,original_name FROM trash_items WHERE batch_id=? ORDER BY created_at,id", batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var item Item
		var parent sql.NullString
		if err := rows.Scan(&item.ID, &item.Type, &item.ItemID, &parent, &item.OriginalName); err != nil {
			return nil, err
		}
		item.OriginalParentID = parent.String
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) restoreConflict(ctx context.Context, tx *sql.Tx, item Item) (bool, error) {
	var count int
	if item.Type == "folder" {
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM folders WHERE state='active' AND COALESCE(parent_id,'')=? AND name=? AND id<>?", item.OriginalParentID, item.OriginalName, item.ItemID).Scan(&count)
		return count > 0, err
	}
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE state='active' AND COALESCE(folder_id,'')=? AND file_name=? AND public_id<>?", item.OriginalParentID, item.OriginalName, item.ItemID).Scan(&count)
	return count > 0, err
}

func canManage(actor auth.Actor, owner string) bool {
	if actor.Kind == auth.ActorAdmin {
		return true
	}
	if actor.Kind == auth.ActorToken {
		_, ok := actor.Scopes["manage"]
		return ok
	}
	return (actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest) && actor.ID == owner
}

func randomID() (string, error) {
	value := make([]byte, 16)
	_, err := rand.Read(value)
	return hex.EncodeToString(value), err
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
