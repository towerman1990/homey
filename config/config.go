package config

import (
	"os"

	"github.com/labstack/gommon/log"
	"gopkg.in/yaml.v3"
)

var (
	GlobalConfig Global
)

type Message struct {
	Format string `yaml:"format"`
	Endian string `yaml:"endian"`
}

type TLV struct {
	TypeByte   int8 `yaml:"type_byte"`
	LengthByte int8 `yaml:"length_byte"`
}

type Config struct {
	WorkerPoolSize   uint32 `yaml:"worker_pool_size"`
	MaxWorkerTaskLen uint32 `yaml:"max_worker_task_len"`
	MaxPackageSize   uint32 `yaml:"max_package_size"`
	Message          `yaml:"message"`
	TLV              `yaml:"tlv"`
}

type Global struct {
	Config `yaml:"config"`
}

func init() {
	loadConfigFile()
}

func loadConfigFile() {
	GlobalConfig = Global{
		Config: Config{
			WorkerPoolSize:   0,
			MaxWorkerTaskLen: 0,
			MaxPackageSize:   4096,
			Message: Message{
				Format: "binary",
				Endian: "little",
			},
			TLV: TLV{
				TypeByte:   0,
				LengthByte: 0,
			},
		},
	}

	configFile, err := os.ReadFile("../example/conf/homey.yml")
	if err != nil {
		log.Errorf("load config file failed, error: %v", err)
		return
	}

	err = yaml.Unmarshal(configFile, &GlobalConfig)
	if err != nil {
		log.Errorf("unmarshal config data failed, error: %v", err)
		return
	}

	if GlobalConfig.Message.Format != "binary" {
		GlobalConfig.TLV.TypeByte = 0
		GlobalConfig.TLV.LengthByte = 0
	}
}
