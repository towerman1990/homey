package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	cfg, err := LoadFile("../example/server/conf/homey.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerPoolSize != 12 || cfg.MaxWorkerTaskLen != 12 || cfg.Message.Format != "text" {
		t.Fatalf("unexpected example configuration: %+v", cfg)
	}
	if cfg.Redis != Default().Redis {
		t.Fatal("omitted Redis configuration should preserve defaults")
	}
}

func TestLoadFileRejectsInvalidConfig(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown key":                "config: {}",
		"invalid endian":             "message: {endian: middle}",
		"binary header in text mode": "tlv: {type: true}",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "homey.yaml")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadFile(path); err == nil {
				t.Fatal("expected invalid configuration to be rejected")
			}
		})
	}
}
