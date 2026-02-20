package main

import (
	"fmt"
	"log"
	"net"

	pb "mygame/proto"
	"mygame/server/match-service/internal/dao"
	"mygame/server/match-service/internal/handler"
	"mygame/server/match-service/pkg/config"

	"google.golang.org/grpc"
)

func main() {
	// 1. 加载配置
	config.InitConfig()

	// 2. 初始化 Redis
	dao.InitRedis()

	// 3. 启动 gRPC 服务
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.AppConfig.Server.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterMatchServiceServer(s, &handler.MatchService{})

	log.Printf("Match Service listening on :%d", config.AppConfig.Server.Port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
