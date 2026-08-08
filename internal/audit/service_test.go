package audit

import "testing"

func TestRedactRemovesSensitiveValues(t *testing.T) {
	redacted := Redact(map[string]any{"token": "secret", "name": "visible"})
	if redacted["token"] != "[REDACTED]" || redacted["name"] != "visible" {
		t.Fatalf("结果 = %#v", redacted)
	}
}
