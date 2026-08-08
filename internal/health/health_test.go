package health

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReadyFailsWhenMasterKeyMissing(t *testing.T) {
	checker := New(Deps{DB: healthyPinger{}, MasterKeyPath: filepath.Join(t.TempDir(), "missing"), DataDir: t.TempDir()})
	if result := checker.Ready(context.Background()); result.OK {
		t.Fatal("主密钥缺失时不应就绪")
	}
}

type healthyPinger struct{}

func (healthyPinger) PingContext(context.Context) error { return nil }
