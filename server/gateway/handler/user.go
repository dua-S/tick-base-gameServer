package handlers

import (
	"context"
	"net/http"
	"time"

	pb "mygame/proto"
	"mygame/server/gateway/rpc"

	"github.com/gin-gonic/gin"
)

// Login
func HandleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 调用 User Service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.UserClient.Login(ctx, &pb.LoginReq{
		Username: req.Username,
		Password: req.Password,
	})

	if err != nil {
		// 实际项目中应解析 gRPC error code 返回更准确的 HTTP 状态码
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":    resp.Token,
		"uid":      resp.Uid,
		"username": resp.Username,
	})
}

// Register
func HandleRegister(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.UserClient.Register(ctx, &pb.RegisterReq{
		Username: req.Username,
		Password: req.Password,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Register failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"uid": resp.Uid})
}

// Get History
func HandleGetHistory(c *gin.Context) {
	uid, _ := c.Get("uid") // 从 Auth Middleware 获取

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.UserClient.GetHistory(ctx, &pb.GetHistoryReq{
		Uid:   uid.(int64),
		Page:  1,
		Limit: 10,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fetch history failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": resp.History})
}
