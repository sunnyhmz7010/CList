package main

import "testing"

func TestDefaultDataDir(t *testing.T) {
	if got := defaultDataDir(); got != "/data" {
		t.Fatalf("defaultDataDir() = %q", got)
	}
}
