package handlers

import (
	"context"
	"net/http"
	"time"

	pb "mygame/proto"
	"mygame/server/gateway/rpc"

	"github.com/gin-gonic/gin"
)

// Create Room
func HandleCreateRoom(c *gin.Context) {
	uid, _ := c.Get("uid")

	var req struct {
		MapId      int32 `json:"map_id"`
		MaxPlayers int32 `json:"max_players"`
	}
	c.ShouldBindJSON(&req)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.MatchClient.CreateRoom(ctx, &pb.CreateRoomReq{
		Uid: uid.(int64),
		Config: &pb.RoomConfig{
			MapId:      req.MapId,
			MaxPlayers: req.MaxPlayers,
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Create room failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room_id":     resp.RoomId,
		"server_ip":   resp.ServerIp,
		"server_port": resp.ServerPort,
		"ticket":      resp.RoomToken,
	})
}

// List Rooms
func HandleListRooms(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.MatchClient.ListRooms(ctx, &pb.ListRoomsReq{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "List rooms failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rooms": resp.Rooms})
}

// Join Room
func HandleJoinRoom(c *gin.Context) {
	uid, _ := c.Get("uid")

	var req struct {
		RoomId string `json:"room_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.MatchClient.JoinRoom(ctx, &pb.JoinRoomReq{
		RoomId: req.RoomId,
		Uid:    uid.(int64),
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Join room failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room_id":     resp.RoomId,
		"server_ip":   resp.ServerIp,
		"server_port": resp.ServerPort,
		"ticket":      resp.RoomToken,
	})
}
