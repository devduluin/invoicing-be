package database

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"

	"duluin_invoice/config"
)

var Redis *redis.Client

// ConnectRedis is best-effort: the service still runs without Redis, it just
// validates every request against SSO instead of using the token cache.
func ConnectRedis() error {
	Redis = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.AppConfig.Redis_Host, config.AppConfig.Redis_Port),
		Password: config.AppConfig.Redis_Password,
		DB:       config.AppConfig.Redis_DB,
	})

	if _, err := Redis.Ping(context.Background()).Result(); err != nil {
		_ = Redis.Close()
		Redis = nil
		return fmt.Errorf("redis not available: %w", err)
	}
	log.Println("✅ Redis connected")
	return nil
}

func CloseRedis() {
	if Redis != nil {
		_ = Redis.Close()
	}
}
