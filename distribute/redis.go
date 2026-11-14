package distribute

import (
	"context"
	"encoding/base64"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/towerman1990/homey/config"
)

var (
	defaultClientOnce sync.Once
	defaultClient     *redis.Client
	WorldChannel      = config.Global.Redis.WorldChannel
	ForwardChannel    = config.Global.Redis.ForwardChannel
)

func NewClient(cfg config.Redis) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:                  cfg.Addr,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		ContextTimeoutEnabled: true,
	})
}

func GetRedisClient() *redis.Client {
	defaultClientOnce.Do(func() {
		defaultClient = NewClient(config.Global.Redis)
	})
	return defaultClient
}

func Publish(ctx context.Context, client *redis.Client, channel string, data []byte) error {
	return client.Publish(ctx, channel, base64.StdEncoding.EncodeToString(data)).Err()
}

func PublishWorldMsg(ctx context.Context, data []byte) error {
	return Publish(ctx, GetRedisClient(), WorldChannel, data)
}

func PublishForwardMsg(ctx context.Context, data []byte) error {
	return Publish(ctx, GetRedisClient(), ForwardChannel, data)
}
