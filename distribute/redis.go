package distribute

import (
	"context"
	"encoding/base64"
	"os"

	log "github.com/towerman1990/homey/logger"

	"github.com/go-redis/redis/v9"
	"github.com/towerman1990/homey/config"
	"go.uber.org/zap"
)

var (
	redisClient    *redis.Client
	WorldChannel   string
	ForwardChannel string
)

func init() {
	WorldChannel = config.Global.Redis.WorldChannel
	ForwardChannel = config.Global.Redis.ForwardChannel

	redisClient = redis.NewClient(&redis.Options{
		Addr:     config.Global.Redis.Addr,
		Password: config.Global.Redis.Password,
		DB:       config.Global.Redis.DB,
	})

	if config.Global.Distribute.Status {
		if statusCmd := redisClient.Ping(context.Background()); statusCmd.Err() != nil {
			log.Logger.Error("failed to ping redis server", zap.String("error", statusCmd.Err().Error()))
			os.Exit(1)
		}
	}
}

func GetRedisClient() *redis.Client {
	return redisClient
}

func PublishWorldMsg(ctx context.Context, data []byte) (err error) {
	_, err = redisClient.Publish(ctx, WorldChannel, base64.StdEncoding.EncodeToString(data)).Result()
	return
}

func PublishForwardMsg(ctx context.Context, data []byte) (err error) {
	_, err = redisClient.Publish(ctx, ForwardChannel, base64.StdEncoding.EncodeToString(data)).Result()
	return
}
