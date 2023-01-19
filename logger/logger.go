package logger

import (
	"log"
	"os"

	"github.com/homey/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func init() {
	encoder := getEncoder()
	sync := getWriteSync()

	var level zapcore.Level
	if config.GlobalConfig.Config.Env == "dev" || config.GlobalConfig.Config.Env == "develop" {
		level = zapcore.DebugLevel
	} else {
		level = zapcore.InfoLevel
	}

	core := zapcore.NewCore(encoder, sync, level)
	Logger = zap.New(core)

	Logger.Info("init logger success")
}

func getEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
}

func getWriteSync() zapcore.WriteSyncer {
	filename := "./log/logs.txt"
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		log.Printf("open log file [%s] failed, error: %v", filename, err)
	}

	syncFile := zapcore.AddSync(file)
	syncConsole := zapcore.AddSync(os.Stderr)

	return zapcore.NewMultiWriteSyncer(syncConsole, syncFile)
}
