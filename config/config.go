package config

import (
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
	Status bool   `yaml:"status"`
	Way    string `yaml:"way"`
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

func loadConfigFile() {
	Global = GlobalConfig{
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
			Addr:           "localhost:6379:",
			Password:       "",
			DB:             0,
			WorldChannel:   "world_channel",
			ForwardChannel: "forward_channel",
		},
	}

	configFile, err := os.ReadFile("./conf/homey.yaml")
	if err != nil {
		log.Printf("load config file failed, error: %v", err)
		return
	}

	err = yaml.Unmarshal(configFile, &Global)
	if err != nil {
		log.Printf("unmarshal config data failed, error: %v", err)
		return
	}
}
