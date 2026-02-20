package main

import (
	"fmt"
	handlers "mygame/server/gateway/handler"
	"mygame/server/gateway/middleware"
	"mygame/server/gateway/pkg/config"
	"mygame/server/gateway/rpc"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化配置
	config.InitConfig()

	// 2. 初始化 RPC 客户端
	rpc.InitClients()

	// 3. 设置 Gin
	if config.AppConfig.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// 4. 全局中间件
	r.Use(middleware.Cors())

	// 5. 路由注册
	api := r.Group("/api")
	{
		// 鉴权模块
		auth := api.Group("/auth")
		{
			auth.POST("/login", handlers.HandleLogin)
			auth.POST("/register", handlers.HandleRegister)
		}

		// 用户模块 (需要登录)
		user := api.Group("/user")
		user.Use(middleware.AuthMiddleware())
		{
			user.GET("/history", handlers.HandleGetHistory)
		}

		// 比赛模块 (需要登录)
		match := api.Group("/match")
		match.Use(middleware.AuthMiddleware())
		{
			match.POST("/create", handlers.HandleCreateRoom)
			match.GET("/rooms", handlers.HandleListRooms)
			match.POST("/join", handlers.HandleJoinRoom)
		}
	}

	// 6. 启动服务
	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	fmt.Printf("Gateway running on %s\n", addr)
	r.Run(addr)
}
