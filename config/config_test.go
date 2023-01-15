package config

import (
	"testing"
)

func TestDataPack(t *testing.T) {
	if GlobalConfig.WorkerPoolSize != 12 {
		t.Errorf("expected be 12, but %d got", GlobalConfig.WorkerPoolSize)
	}

	if GlobalConfig.Message.Format != "text" {
		t.Errorf("expected be text, but %s got", GlobalConfig.Message.Format)
	}

	if GlobalConfig.TLV.TypeByte == 1 {
		t.Errorf("expected be 0, but %d got", GlobalConfig.TLV.TypeByte)
	}
}
