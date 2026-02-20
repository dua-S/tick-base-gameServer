package dao

import (
	"context"
	"log"
	"time"

	"mygame/server/game-service/pkg/config"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

const KeyRoomPrefix = "room:"

func InitRedis() {
	cfg := config.AppConfig.Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := RDB.Ping(ctx).Result(); err != nil {
		log.Fatalf("Redis connect failed: %v", err)
	}
}

func GetRoom(ctx context.Context, roomID string) (map[string]string, error) {
	return RDB.HGetAll(ctx, KeyRoomPrefix+roomID).Result()
}

// ValidateRoomToken checks whether the given token matches the stored room token.
func ValidateRoomToken(ctx context.Context, roomID, token string) (bool, error) {
	val, err := RDB.HGet(ctx, KeyRoomPrefix+roomID, "token").Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == token, nil
}
