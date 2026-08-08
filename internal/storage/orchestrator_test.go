package storage

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/jobs"
)

func TestTimeoutAfterTelegramSendMarksUncertain(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	registry.Register("tg", deadlineBackend{})
	orchestrator := NewOrchestrator(registry, jobs.NewStore(database))
	result := orchestrator.Put(context.Background(), "tg", strings.NewReader("x"), ObjectMeta{FileName: "x.txt"})
	if result.Job.State != jobs.Uncertain || result.Job.Attempts != 1 || !errors.Is(result.Err, jobs.ErrUncertain) {
		t.Fatalf("结果 = %+v", result)
	}
}

type deadlineBackend struct{}

func (deadlineBackend) Validate(context.Context) error    { return nil }
func (deadlineBackend) Capabilities() Capabilities        { return Capabilities{Streaming: true} }
func (deadlineBackend) HealthCheck(context.Context) error { return nil }
func (deadlineBackend) Put(context.Context, io.Reader, ObjectMeta) (Object, error) {
	return Object{}, context.DeadlineExceeded
}
func (deadlineBackend) Open(context.Context, string, *ByteRange) (Reader, error) {
	return Reader{}, nil
}
func (deadlineBackend) Delete(context.Context, string) error { return nil }
