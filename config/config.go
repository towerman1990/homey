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
	T int8 `yaml:"type"`
	L int8 `yaml:"length"`
}

type Config struct {
	WorkerPoolSize   uint32 `yaml:"worker_pool_size"`
	MaxWorkerTaskLen uint32 `yaml:"max_worker_task_len"`
	MaxPackageSize   uint32 `yaml:"max_package_size"`
	Message          `yaml:"message"`
	TLV
}

type Global struct {
	Config `yaml:"config"`
}

func init() {
	log.Info("config init")
	loadConfigFile()
}

func loadConfigFile() {
	log.Info("call loadConfigFile function")
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
				T: 0,
				L: 0,
			},
		},
	}

	configFile, err := os.ReadFile("./conf/homey.yml")
	if err != nil {
		log.Errorf("load config file failed, error: %v", err)
		return
	}

	err = yaml.Unmarshal(configFile, &GlobalConfig)
	if err != nil {
		log.Errorf("unmarshal config data failed, error: %v", err)
		return
	}

	log.Info(GlobalConfig.WorkerPoolSize)
	log.Info(GlobalConfig.Message.Format)
	if GlobalConfig.Message.Format != "binary" {
		GlobalConfig.TLV.T = 0
		GlobalConfig.TLV.L = 0
	}
}
