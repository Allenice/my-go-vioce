//go:build darwin

package keychain

import (
	"context"
	"testing"
)

func TestFrameworkBindingsWithoutAccessingCredentials(t *testing.T) {
	q, e := query(context.Background(), "https://unit-test.invalid/v1")
	if e != nil {
		t.Fatal(e)
	}
	if q == 0 {
		t.Fatal("no query")
	}
	native.release(q)
}
func TestRejectInvalidBeforeCallingKeychain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := (Store{}).Get(ctx, "account"); e == nil {
		t.Fatal("cancelled read")
	}
	if e := (Store{}).Set(context.Background(), "account", ""); e == nil {
		t.Fatal("empty credential")
	}
}
