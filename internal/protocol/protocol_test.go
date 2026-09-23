package protocol

import (
	"bytes"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	msg, err := Encode(ServerChat, ChatMsg{ID: 7, Msg: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	// The wire format is a 2 element array [type, payload]
	var arr []msgpack.RawMessage
	if err := msgpack.Unmarshal(msg, &arr); err != nil || len(arr) != 2 {
		t.Fatalf("not a [type, payload] array: %v %d", err, len(arr))
	}

	typ, data, err := Decode(msg)
	if err != nil || ServerMsg(typ) != ServerChat {
		t.Fatalf("decode: %d %v", typ, err)
	}
	var chat ChatMsg
	if err := msgpack.Unmarshal(data, &chat); err != nil || chat != (ChatMsg{ID: 7, Msg: "hi"}) {
		t.Fatalf("payload: %+v %v", chat, err)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	for _, bad := range [][]byte{nil, {0xc1}, {0x92, 0x01}} {
		if _, _, err := Decode(bad); err == nil {
			t.Errorf("% x decoded without error", bad)
		}
	}
}

func TestEntityMoveIsCompact(t *testing.T) {
	b, err := marshal(EntityMove{ID: 1, X: 2, Y: 3})
	if err != nil {
		t.Fatal(err)
	}
	// fixarray of 3 positive fixints
	if !bytes.Equal(b, []byte{0x93, 0x01, 0x02, 0x03}) {
		t.Fatalf("EntityMove should encode as [id, x, y], got % x", b)
	}
}

func TestEmptyWorldUpdateOmitsFields(t *testing.T) {
	b, _ := msgpack.Marshal(WorldUpdate{Move: []EntityMove{{ID: 1}}})
	var m map[string]any
	msgpack.Unmarshal(b, &m)
	if _, ok := m["spawn"]; ok || len(m) != 1 {
		t.Fatalf("empty lists should be left out: %v", m)
	}
}

func TestMessagesUseCompactInts(t *testing.T) {
	msg, _ := Encode(ServerWorld, WorldUpdate{Move: []EntityMove{{ID: 300, X: 4000, Y: -5}}})
	// [5, {"move": [[300, 4000, -5]]}]: uint16, uint16, negative fixint
	want := []byte{0x92, 0x05, 0x81, 0xa4, 'm', 'o', 'v', 'e', 0x91, 0x93, 0xcd, 0x01, 0x2c, 0xcd, 0x0f, 0xa0, 0xfb}
	if !bytes.Equal(msg, want) {
		t.Fatalf("got  % x\nwant % x", msg, want)
	}
}

// Slices of small int types encode as msgpack binary, which the client would get
// as a Uint8Array instead of an array. Enums inside slices must be int.
func TestWordleColorsAreArrays(t *testing.T) {
	b, _ := msgpack.Marshal(WordleRes{Colors: []WordleColor{Green, Yellow}})
	var m map[string]any
	msgpack.Unmarshal(b, &m)
	if _, ok := m["colors"].([]any); !ok {
		t.Fatalf("colors should be an array, got %T", m["colors"])
	}
}
