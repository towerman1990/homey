package service

import (
	"github.com/go-redis/redis/v9"
	"github.com/homey/config"
)

var redisClient *redis.Client

func init() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     config.GlobalConfig.Redis.Addr,
		Password: config.GlobalConfig.Redis.Password,
		DB:       config.GlobalConfig.Redis.DB,
	})
}

func GetRedisClient() *redis.Client {
	return redisClient
}
