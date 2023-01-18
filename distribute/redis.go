package distribute

import (
	"context"
	"encoding/base64"

	"github.com/homey/config"
	"github.com/homey/service"
)

var (
	WorldChannel   string
	ForwardChannel string
)

func init() {
	WorldChannel = config.GlobalConfig.Redis.WorldChannel
	ForwardChannel = config.GlobalConfig.Redis.ForwardChannel
}

func PublishWorldMsg(context context.Context, data []byte) (err error) {
	rdb := service.GetRedisClient()
	_, err = rdb.Publish(context, WorldChannel, base64.StdEncoding.EncodeToString(data)).Result()

	return
}

func PublishForwardMsg(context context.Context, data []byte) (err error) {
	rdb := service.GetRedisClient()
	_, err = rdb.Publish(context, ForwardChannel, base64.StdEncoding.EncodeToString(data)).Result()

	return
}
