package config

import (
	"testing"
)

func TestConfig(t *testing.T) {
	if GlobalConfig.WorkerPoolSize != 12 {
		t.Errorf("expected be 12, but %d got", GlobalConfig.WorkerPoolSize)
	}

	if GlobalConfig.Message.Format != "binary" {
		t.Errorf("expected be text, but %s got", GlobalConfig.Message.Format)
	}

}
