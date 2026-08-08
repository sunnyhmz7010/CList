package contract

import (
	"context"
	"crypto/sha256"
	"io"
	"strings"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/storage"
	"github.com/sunnyhmz7010/CList/internal/storage/local"
)

func TestBackendContract(t *testing.T) {
	backends := []struct {
		name string
		new  func(t *testing.T) storage.Backend
	}{
		{"local", newLocalBackend},
		{"official-mock", func(*testing.T) storage.Backend {
			return newMemoryBackend(storage.Capabilities{Range: true, Head: true, Streaming: true})
		}},
		{"streaming-mock", func(*testing.T) storage.Backend { return newMemoryBackend(storage.Capabilities{Streaming: true}) }},
	}
	for _, testCase := range backends {
		t.Run(testCase.name, func(t *testing.T) { runBackendContract(t, testCase.new(t)) })
	}
}

func newLocalBackend(t *testing.T) storage.Backend { return local.New(t.TempDir()) }

func runBackendContract(t *testing.T, backend storage.Backend) {
	ctx := context.Background()
	if err := backend.Validate(ctx); err != nil {
		t.Fatal(err)
	}
	content := "contract-content"
	want := sha256.Sum256([]byte(content))
	object, err := backend.Put(ctx, strings.NewReader(content), storage.ObjectMeta{Key: "contract/key", FileName: "contract.txt", MIMEType: "text/plain", Size: int64(len(content))})
	if err != nil {
		t.Fatal(err)
	}
	if object.Size != int64(len(content)) || object.SHA256 != fmtHex(want[:]) {
		t.Fatalf("对象 = %+v", object)
	}
	reader, err := backend.Open(ctx, object.Key, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(raw) != content {
		t.Fatalf("读取 = %q, %v", raw, err)
	}
	if err := backend.Delete(ctx, object.Key); err != nil {
		t.Fatal(err)
	}
}

type memoryBackend struct {
	capabilities storage.Capabilities
	data         map[string][]byte
}

func newMemoryBackend(capabilities storage.Capabilities) *memoryBackend {
	return &memoryBackend{capabilities: capabilities, data: map[string][]byte{}}
}
func (b *memoryBackend) Validate(context.Context) error     { return nil }
func (b *memoryBackend) Capabilities() storage.Capabilities { return b.capabilities }
func (b *memoryBackend) HealthCheck(context.Context) error  { return nil }
func (b *memoryBackend) Put(_ context.Context, reader io.Reader, meta storage.ObjectMeta) (storage.Object, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return storage.Object{}, err
	}
	sum := sha256.Sum256(raw)
	b.data[meta.Key] = raw
	return storage.Object{Key: meta.Key, Size: int64(len(raw)), SHA256: fmtHex(sum[:])}, nil
}
func (b *memoryBackend) Open(_ context.Context, key string, byteRange *storage.ByteRange) (storage.Reader, error) {
	raw, ok := b.data[key]
	if !ok {
		return storage.Reader{}, storage.ErrObjectNotFound
	}
	if byteRange != nil {
		if !b.capabilities.Range {
			return storage.Reader{}, storage.ErrRangeUnsupported
		}
		raw = raw[byteRange.Start : byteRange.End+1]
	}
	return storage.Reader{ReadCloser: io.NopCloser(strings.NewReader(string(raw))), Size: int64(len(raw)), Partial: byteRange != nil}, nil
}
func (b *memoryBackend) Delete(_ context.Context, key string) error { delete(b.data, key); return nil }
func fmtHex(value []byte) string {
	const hex = "0123456789abcdef"
	var result strings.Builder
	for _, item := range value {
		result.WriteByte(hex[item>>4])
		result.WriteByte(hex[item&15])
	}
	return result.String()
}
