package preview

import (
	"errors"
	"strings"
	"testing"
)

func TestPreviewRejectsUnsafeDocx(t *testing.T) {
	if err := ValidateDocx(strings.NewReader("not-a-zip")); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("错误 = %v", err)
	}
}

func TestKindForPreviewMedia(t *testing.T) {
	for _, item := range []struct {
		mime, name string
		want       Kind
	}{
		{"image/png", "x", KindImage},
		{"video/mp4", "x", KindVideo},
		{"audio/mpeg", "x", KindAudio},
		{"application/pdf", "x", KindPDF},
		{"application/octet-stream", "x.docx", KindDOCX},
		{"text/plain", "x", KindText},
	} {
		if got := kindFor(item.mime, item.name); got != item.want {
			t.Fatalf("%s/%s = %s", item.mime, item.name, got)
		}
	}
}
