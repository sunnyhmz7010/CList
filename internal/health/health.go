package health

import (
	"context"
	"os"
	"path/filepath"
)

type Pinger interface{ PingContext(context.Context) error }
type BackendHealth interface{ HealthCheck(context.Context) error }

type Deps struct {
	DB            Pinger
	Backends      BackendHealth
	MasterKeyPath string
	DataDir       string
}

type Result struct {
	OK     bool              `json:"ok"`
	Checks map[string]string `json:"checks"`
}

func (r Result) Status() int {
	if r.OK {
		return 200
	}
	return 503
}

type Checker struct{ deps Deps }

func New(deps Deps) *Checker { return &Checker{deps: deps} }

func (h *Checker) Ready(ctx context.Context) Result {
	result := Result{OK: true, Checks: make(map[string]string)}
	if h.deps.DB == nil || h.deps.DB.PingContext(ctx) != nil {
		result.OK = false
		result.Checks["sqlite"] = "unavailable"
	} else {
		result.Checks["sqlite"] = "ok"
	}
	if info, err := os.Stat(h.deps.MasterKeyPath); err != nil || !info.Mode().IsRegular() {
		result.OK = false
		result.Checks["master_key"] = "missing"
	} else {
		result.Checks["master_key"] = "ok"
	}
	if err := writable(h.deps.DataDir); err != nil {
		result.OK = false
		result.Checks["data_dir"] = "not_writable"
	} else {
		result.Checks["data_dir"] = "ok"
	}
	if h.deps.Backends != nil {
		if err := h.deps.Backends.HealthCheck(ctx); err != nil {
			result.OK = false
			result.Checks["storage"] = "unavailable"
		} else {
			result.Checks["storage"] = "ok"
		}
	}
	return result
}

func writable(dir string) error {
	file, err := os.CreateTemp(filepath.Clean(dir), ".clist-health-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}
