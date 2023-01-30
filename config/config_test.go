package config

import (
	"testing"
)

func TestConfig(t *testing.T) {
	if Global.WorkerPoolSize != 12 {
		t.Errorf("expected be 12, but %d got", Global.WorkerPoolSize)
	}

	if Global.Message.Format != "binary" {
		t.Errorf("expected be text, but %s got", Global.Message.Format)
	}

}
