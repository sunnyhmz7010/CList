package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestExpiredLeaseReturnsToRetryQueue(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := NewStore(database)
	job, err := store.Enqueue(context.Background(), "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Begin(context.Background(), job.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverExpired(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	job, err = store.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != RetryWait {
		t.Fatalf("状态 = %s", job.State)
	}
}
