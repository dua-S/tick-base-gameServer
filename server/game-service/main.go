package main

import (
	"fmt"
	"log"
	"mygame/server/game-service/internal/core"
	"mygame/server/game-service/internal/dao"
	"mygame/server/game-service/internal/handler"
	"mygame/server/game-service/internal/mq"
	"mygame/server/game-service/pkg/config"

	"github.com/gin-gonic/gin"
)

func main() {
	config.InitConfig()
	// 初始化 Redis（用于房间 ticket 校验）
	dao.InitRedis()

	mq.InitMQ()

	go func() {
		if err := handler.StartGRPC(config.AppConfig.Server.GrpcPort); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/ws", core.HandleWebSocket)

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	fmt.Printf("Game Service running on %s\n", addr)
	r.Run(addr)
}
