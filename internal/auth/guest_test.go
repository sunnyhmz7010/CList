package auth

import (
	"errors"
	"testing"
)

func TestHomeSessionCannotEnterGallery(t *testing.T) {
	home := Actor{Kind: ActorGuest, Scopes: map[string]struct{}{string(ScopeHome): {}}}
	if err := RequireScope(home, ScopeGallery); !errors.Is(err, ErrScopeRequired) {
		t.Fatalf("got %v", err)
	}
	if err := RequireFile(home, "public-1"); !errors.Is(err, ErrScopeRequired) {
		t.Fatalf("got %v", err)
	}
}
