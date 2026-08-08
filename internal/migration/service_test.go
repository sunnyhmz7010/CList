package migration

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCopyAndHashRejectsMismatch(t *testing.T) {
	var destination strings.Builder
	if _, err := copyAndHash(context.Background(), strings.NewReader("source"), &destination, strings.Repeat("0", 64)); !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("错误 = %v", err)
	}
	if destination.String() != "source" {
		t.Fatalf("复制内容 = %q", destination.String())
	}
}
