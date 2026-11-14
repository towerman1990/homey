package network

import (
	"bytes"
	"testing"

	"github.com/towerman1990/homey/config"
)

func TestPackAndUnPackData(t *testing.T) {
	for _, order := range []string{"little", "big"} {
		for _, format := range []struct {
			name               string
			hasType, hasLength bool
		}{
			{"raw", false, false},
			{"type", true, false},
			{"length", false, true},
			{"type and length", true, true},
		} {
			t.Run(order+"/"+format.name, func(t *testing.T) {
				cfg := config.Default()
				cfg.Message.Endian = order
				cfg.TLV.Type, cfg.TLV.Length = format.hasType, format.hasLength
				codec := newMessageCodec(cfg)

				for _, id := range []uint64{0, 123} {
					msg := NewMessage(7, []byte("hello"))
					msg.SetConnID(id)
					packed, err := codec.pack(msg)
					if err != nil {
						t.Fatal(err)
					}
					got, err := codec.unpack(packed, id != 0)
					if err != nil {
						t.Fatal(err)
					}
					wantType := uint32(0)
					if format.hasType {
						wantType = msg.GetDataType()
					}
					if got.GetConnID() != id || got.GetDataType() != wantType ||
						got.GetDataLength() != 5 || !bytes.Equal(got.GetData(), msg.GetData()) {
						t.Fatalf("unexpected decoded message: %+v", got)
					}
				}
			})
		}
	}
}

func TestUnPackRejectsInvalidLength(t *testing.T) {
	cfg := config.Default()
	cfg.TLV.Length = true
	cfg.MaxPackageSize = 2
	codec := newMessageCodec(cfg)
	for name, data := range map[string][]byte{
		"truncated header":  {1, 0},
		"truncated payload": {2, 0, 0, 0, 'a'},
		"trailing bytes":    {1, 0, 0, 0, 'a', 'b'},
		"oversized payload": {3, 0, 0, 0, 'a', 'b', 'c'},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := codec.unpack(data, false); err == nil {
				t.Fatal("expected invalid packet to be rejected")
			}
		})
	}
}
