package main

import (
	"fmt"
	"log"
	"net"

	pb "mygame/proto"
	"mygame/server/user-service/internal/dao"
	"mygame/server/user-service/internal/handler"
	"mygame/server/user-service/internal/mq"
	"mygame/server/user-service/pkg/config"

	"google.golang.org/grpc"
)

func main() {
	// 1. 加载配置
	config.InitConfig()

	// 2. 初始化数据库
	// 构建 DSN: username:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local
	// 目前配置只包含端口、用户名、密码和数据库名，使用本地地址 127.0.0.1
	mysqlCfg := config.AppConfig.MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlCfg.Username, mysqlCfg.Password, mysqlCfg.Port, mysqlCfg.Database)
	dao.InitMySQL(dsn)

	// 3. 初始化 MQ 并启动 Consumer
	mq.InitMQ()
	go mq.StartConsumer()

	// 4. 启动 gRPC 服务
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.AppConfig.Server.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &handler.UserService{})

	log.Printf("User Service listening on :%d", config.AppConfig.Server.Port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
