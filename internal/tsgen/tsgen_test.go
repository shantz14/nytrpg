package tsgen

import (
	"bytes"
	"os"
	"testing"
)

// Fails when internal/protocol changed but the client types weren't regenerated
func TestGeneratedProtocolIsCurrent(t *testing.T) {
	want, err := Generate("../protocol")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../client/src/protocol.gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("client/src/protocol.gen.ts is stale, run `go generate ./internal/protocol`")
	}
}
