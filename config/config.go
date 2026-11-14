package config

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	Global GlobalConfig
)

type Message struct {
	Format string `yaml:"format"`
	Endian string `yaml:"endian"`
}

type TLV struct {
	Type   bool `yaml:"type"`
	Length bool `yaml:"length"`
}

type Distribute struct {
	Status    bool   `yaml:"status"`
	Way       string `yaml:"way"`
	MachineID uint16 `yaml:"machine_id"`
}

type Redis struct {
	Addr           string `yaml:"addr"`
	Password       string `yaml:"password"`
	DB             int    `yaml:"db"`
	WorldChannel   string `yaml:"world_channel"`
	ForwardChannel string `yaml:"forward_channel"`
}

type Framework struct {
	Env              string `yaml:"env"`
	WorkerPoolSize   uint32 `yaml:"worker_pool_size"`
	MaxWorkerTaskLen uint32 `yaml:"max_worker_task_len"`
	MaxPackageSize   uint32 `yaml:"max_package_size"`
}

type GlobalConfig struct {
	Framework  `yaml:"framework"`
	Message    `yaml:"message"`
	TLV        `yaml:"tlv"`
	Distribute `yaml:"distribute"`
	Redis      `yaml:"redis"`
}

func init() {
	loadConfigFile()
}

func Default() GlobalConfig {
	return GlobalConfig{
		Framework: Framework{
			Env:              "develop",
			WorkerPoolSize:   0,
			MaxWorkerTaskLen: 0,
			MaxPackageSize:   4096},
		Message: Message{
			Format: "text",
			Endian: "little",
		},
		TLV: TLV{
			Type:   false,
			Length: false,
		},
		Distribute: Distribute{
			Status: false,
			Way:    "redis",
		},
		Redis: Redis{
			Addr:           "localhost:6379",
			Password:       "",
			DB:             0,
			WorldChannel:   "world_channel",
			ForwardChannel: "forward_channel",
		},
	}
}

func loadConfigFile() {
	Global = Default()
	cfg, err := LoadFile("./conf/homey.yaml")
	if err == nil {
		Global = cfg
	} else if !os.IsNotExist(err) {
		log.Printf("load homey config: %v", err)
	}
}

func LoadFile(path string) (GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GlobalConfig{}, err
	}
	cfg := Default()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return GlobalConfig{}, fmt.Errorf("decode %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return GlobalConfig{}, fmt.Errorf("%s must contain one YAML document", path)
	}
	return cfg, cfg.Validate()
}

func (cfg GlobalConfig) Validate() error {
	if cfg.Message.Format != "text" && cfg.Message.Format != "binary" {
		return fmt.Errorf("invalid message format %q", cfg.Message.Format)
	}
	if cfg.Message.Endian != "little" && cfg.Message.Endian != "big" {
		return fmt.Errorf("invalid message byte order %q", cfg.Message.Endian)
	}
	if cfg.Message.Format == "text" && (cfg.TLV.Type || cfg.TLV.Length) {
		return fmt.Errorf("binary message headers require binary mode")
	}
	if cfg.Distribute.Status && (cfg.Distribute.Way != "redis" || cfg.Redis.Addr == "" ||
		cfg.Redis.WorldChannel == "" || cfg.Redis.ForwardChannel == "") {
		return fmt.Errorf("distribution requires a Redis address and channel names")
	}
	if cfg.Distribute.Status && cfg.Distribute.MachineID == 0 {
		return fmt.Errorf("distribution requires a nonzero machine_id")
	}
	return nil
}
