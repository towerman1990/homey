package network

import (
	"encoding/binary"
	"testing"

	"github.com/towerman1990/homey/config"
)

func TestPackAndUnPackData(t *testing.T) {
	// test message pack & unpack use little endian
	t.Log(config.Global.TLV.Type)
	t.Log(config.Global.TLV.Length)
	t.Log(config.Global.Message.Endian)
	t.Log(config.Global.Message.Format)

	const content = "Hello World!"
	message := NewMessage(1, []byte(content))
	t.Log(message)
	packageData, err := Pack(message)
	if err != nil {
		t.Errorf("pack message error: %v", err)
	}
	t.Log(packageData)

	message, err = UnPack(packageData, false)
	if err != nil {
		t.Errorf("unpack message error: %v", err)
	}
	t.Log(message)

	if string(message.GetData()) != content {
		t.Error("data pack fail")
	}

	// test forward message pack & unpack use big endian
	endian = binary.BigEndian

	message = NewMessage(2, []byte(content))
	message.SetConnID(1)
	t.Log(message)
	packageData, err = Pack(message)
	if err != nil {
		t.Errorf("pack message error: %v", err)
	}
	t.Log(packageData)

	message, err = UnPack(packageData, true)
	if err != nil {
		t.Errorf("unpack message error: %v", err)
	}
	t.Log(message)

	if string(message.GetData()) != content {
		t.Error("data pack fail")
	} else {
		t.Logf("message type: %d", message.GetDataType())
		t.Logf("message content: %s", message.GetData())
	}
}
