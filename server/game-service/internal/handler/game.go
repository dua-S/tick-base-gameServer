package handler

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "mygame/proto"
	"mygame/server/game-service/internal/core"
	"mygame/server/game-service/internal/dao"

	"google.golang.org/grpc"
)

type GameServiceServer struct {
	pb.UnimplementedGameServiceServer
}

func (s *GameServiceServer) ValidateToken(ctx context.Context, req *pb.GameValidateTokenReq) (*pb.GameValidateTokenResp, error) {
	roomID := req.RoomId
	token := req.Token

	log.Printf("Validating token for room %s", roomID)

	room := core.GetRoom(roomID)
	if room == nil {
		return &pb.GameValidateTokenResp{
			Valid: false,
		}, nil
	}

	valid, err := dao.ValidateRoomToken(ctx, roomID, token)
	if err != nil {
		return nil, err
	}
	return &pb.GameValidateTokenResp{
		Valid: valid,
	}, nil
}

func (s *GameServiceServer) NotifyGameStart(ctx context.Context, req *pb.NotifyGameStartReq) (*pb.NotifyGameStartResp, error) {
	roomID := req.RoomId

	log.Printf("Notifying game start for room %s", roomID)

	room := core.GetRoom(roomID)
	if room == nil {
		return &pb.NotifyGameStartResp{Success: false}, nil
	}

	core.StartRoom(roomID)
	return &pb.NotifyGameStartResp{Success: true}, nil
}

func StartGRPC(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	pb.RegisterGameServiceServer(s, &GameServiceServer{})

	log.Printf("Game Service gRPC listening on :%d", port)
	return s.Serve(lis)
}
