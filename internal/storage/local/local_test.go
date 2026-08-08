package local

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/storage"
)

func TestLocalPutUsesStableKeyAndRejectsEscape(t *testing.T) {
	root := t.TempDir()
	backend := New(root)
	object, err := backend.Put(context.Background(), strings.NewReader("hello"), storage.ObjectMeta{Key: "objects/a"})
	if err != nil || object.Key != "objects/a" {
		t.Fatalf("object=%+v err=%v", object, err)
	}
	if _, err := backend.Open(context.Background(), "../secret", nil); !errors.Is(err, storage.ErrPathEscape) {
		t.Fatalf("got %v", err)
	}
}
