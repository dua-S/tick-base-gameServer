package rpc

import (
	"log"
	pb "mygame/proto"                  // 替换为你的 pb 路径
	"mygame/server/gateway/pkg/config" // 替换为你的实际 module name

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	UserClient  pb.UserServiceClient
	MatchClient pb.MatchServiceClient
)

func InitClients() {
	// 1. 连接 User Service
	connUser, err := grpc.Dial(config.AppConfig.RPC.UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to user-service: %v", err)
	}
	UserClient = pb.NewUserServiceClient(connUser)
	log.Printf("Connected to User Service at %s", config.AppConfig.RPC.UserServiceAddr)

	// 2. 连接 Match Service
	connMatch, err := grpc.Dial(config.AppConfig.RPC.MatchServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to match-service: %v", err)
	}
	MatchClient = pb.NewMatchServiceClient(connMatch)
	log.Printf("Connected to Match Service at %s", config.AppConfig.RPC.MatchServiceAddr)
}
