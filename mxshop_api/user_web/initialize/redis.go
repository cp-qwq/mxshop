package initialize

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"mxshop_api/user_web/global"
)

func InitRedisClient() {
	global.RedisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", global.ServerConfig.RedisInfo.Host, global.ServerConfig.RedisInfo.Port),
	})
}