package handler

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	pb "mygame/proto"
	"mygame/server/match-service/internal/dao"
	"mygame/server/match-service/pkg/config"

	"github.com/google/uuid"
)

type MatchService struct {
	pb.UnimplementedMatchServiceServer
}

// CreateRoom 创建房间
func (s *MatchService) CreateRoom(ctx context.Context, req *pb.CreateRoomReq) (*pb.CreateRoomResp, error) {
	// 1. 简单的调度算法：随机选择一个 Game Server
	// 实际项目中这里应该去查 Redis 里的服务器负载信息
	servers := config.AppConfig.GameServers
	if len(servers) == 0 {
		return nil, fmt.Errorf("no available game servers")
	}
	targetServer := servers[rand.Intn(len(servers))]

	// 2. 生成房间 ID 和 Token
	roomID := uuid.New().String()
	token := uuid.New().String()

	roomName := "Room " + roomID[:8]
	if req.Config != nil && req.Config.RoomName != "" {
		roomName = req.Config.RoomName
	}

	maxPlayers := int32(8)
	if req.Config != nil && req.Config.MaxPlayers > 0 {
		maxPlayers = req.Config.MaxPlayers
	}

	// 3. 保存到 Redis
	err := dao.SaveRoom(ctx, roomID, map[string]interface{}{
		"room_name":       roomName,
		"max_players":     maxPlayers,
		"current_players": 0,
		"status":          "WAITING",
		"server_ip":       targetServer.IP,
		"server_port":     targetServer.Port,
		"token":           token,
		"creator_uid":     req.Uid,
		"created_at":      time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}

	// 4. 返回给 Gateway -> Client
	return &pb.CreateRoomResp{
		RoomId:     roomID,
		ServerIp:   targetServer.IP,
		ServerPort: int32(targetServer.Port),
		RoomToken:  token, // Client 拿着这个去连 WS
	}, nil
}

// ListRooms 获取列表
func (s *MatchService) ListRooms(ctx context.Context, req *pb.ListRoomsReq) (*pb.ListRoomsResp, error) {
	data, err := dao.GetAllRooms(ctx)
	if err != nil {
		return nil, err
	}

	var pbRooms []*pb.RoomInfo
	for _, r := range data {
		// Redis HGetAll 返回的是 string，需要转换
		cur, _ := strconv.Atoi(r["current_players"])
		max, _ := strconv.Atoi(r["max_players"])

		pbRooms = append(pbRooms, &pb.RoomInfo{
			RoomId:         r["room_id"],
			CurrentPlayers: int32(cur),
			MaxPlayers:     int32(max),
			Status:         r["status"],
		})
	}

	return &pb.ListRoomsResp{Rooms: pbRooms}, nil
}

func (s *MatchService) JoinRoom(ctx context.Context, req *pb.JoinRoomReq) (*pb.JoinRoomResp, error) {
	roomData, err := dao.GetRoom(ctx, req.RoomId)
	if err != nil || len(roomData) == 0 {
		return nil, fmt.Errorf("room not found or expired")
	}

	curPlayers, _ := strconv.Atoi(roomData["current_players"])
	maxPlayers, _ := strconv.Atoi(roomData["max_players"])

	if curPlayers >= maxPlayers {
		return nil, fmt.Errorf("room is full")
	}

	status, _ := roomData["status"]
	if status != "WAITING" {
		return nil, fmt.Errorf("room is not available, status: %s", status)
	}

	token := uuid.New().String()
	err = dao.UpdateRoom(ctx, req.RoomId, map[string]interface{}{
		"current_players": curPlayers + 1,
		"token":           token,
	})
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(roomData["server_port"])
	if port == 0 {
		return nil, fmt.Errorf("invalid server port")
	}

	return &pb.JoinRoomResp{
		RoomId:     req.RoomId,
		ServerIp:   roomData["server_ip"],
		ServerPort: int32(port),
		RoomToken:  token,
	}, nil
}

func (s *MatchService) UpdateRoom(ctx context.Context, req *pb.UpdateRoomReq) (*pb.UpdateRoomResp, error) {
	// 获取房间信息
	roomData, err := dao.GetRoom(ctx, req.RoomId)
	if err != nil || len(roomData) == 0 {
		return nil, fmt.Errorf("room not found")
	}

	// 验证UID是否匹配creator_uid（只有房主可以更新）
	creatorUid, _ := strconv.ParseInt(roomData["creator_uid"], 10, 64)
	if creatorUid != req.Uid {
		return &pb.UpdateRoomResp{
			Success: false,
			Message: "only room host can update room settings",
		}, nil
	}

	// 验证房间状态
	status := roomData["status"]
	if status != "WAITING" {
		return &pb.UpdateRoomResp{
			Success: false,
			Message: "room can only be updated when in WAITING status",
		}, nil
	}

	// 构建更新字段
	updateFields := make(map[string]interface{})
	if req.Config != nil {
		if req.Config.RoomName != "" {
			updateFields["room_name"] = req.Config.RoomName
		}
		if req.Config.MaxPlayers > 0 {
			updateFields["max_players"] = req.Config.MaxPlayers
		}
	}

	// 执行更新
	if len(updateFields) > 0 {
		if err := dao.UpdateRoom(ctx, req.RoomId, updateFields); err != nil {
			return nil, fmt.Errorf("failed to update room: %v", err)
		}
	}

	return &pb.UpdateRoomResp{
		Success: true,
		Message: "room updated successfully",
	}, nil
}
