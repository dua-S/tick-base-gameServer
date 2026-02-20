package dao

import (
	"context"
	"log"
	"time"

	"mygame/server/match-service/pkg/config"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

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

// 键名定义
const (
	KeyRoomList   = "rooms:available" // Set: 存储 room_id
	KeyRoomPrefix = "room:"           // Hash: room:{id} -> { details }
)

// SaveRoom 创建房间
func SaveRoom(ctx context.Context, roomID string, data map[string]interface{}) error {
	pipe := RDB.Pipeline()

	// 1. 存入房间详情
	key := KeyRoomPrefix + roomID
	pipe.HMSet(ctx, key, data)
	pipe.Expire(ctx, key, 24*time.Hour) // 设置过期时间防止死数据

	// 2. 加入列表
	pipe.SAdd(ctx, KeyRoomList, roomID)

	_, err := pipe.Exec(ctx)
	return err
}

// GetRoom 获取单个房间信息
func GetRoom(ctx context.Context, roomID string) (map[string]string, error) {
	return RDB.HGetAll(ctx, KeyRoomPrefix+roomID).Result()
}

// GetAllRooms 获取所有房间列表 (高频操作，生产环境需分页或用 Scan)
func GetAllRooms(ctx context.Context) ([]map[string]string, error) {
	// 1. 获取所有 ID
	ids, err := RDB.SMembers(ctx, KeyRoomList).Result()
	if err != nil {
		return nil, err
	}

	var rooms []map[string]string
	// 2. 遍历获取详情 (可优化为 Pipeline)
	// 简单实现：循环获取
	for _, id := range ids {
		data, err := RDB.HGetAll(ctx, KeyRoomPrefix+id).Result()
		if err == nil && len(data) > 0 {
			// 把 ID 也塞回去方便前端用
			data["room_id"] = id
			rooms = append(rooms, data)
		}
	}
	return rooms, nil
}

// RemoveRoom 销毁房间
func RemoveRoom(ctx context.Context, roomID string) error {
	pipe := RDB.Pipeline()
	pipe.Del(ctx, KeyRoomPrefix+roomID)
	pipe.SRem(ctx, KeyRoomList, roomID)
	_, err := pipe.Exec(ctx)
	return err
}

func UpdateRoom(ctx context.Context, roomID string, data map[string]interface{}) error {
	key := KeyRoomPrefix + roomID
	return RDB.HMSet(ctx, key, data).Err()
}
