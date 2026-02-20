package core

import (
	pb "mygame/proto"
)

type Player struct {
	UID      int64
	Username string
	Conn     *WebSocketConn // 网络连接封装

	// 物理属性
	X, Y   float64
	Speed  float64
	HP     int32
	MaxHP  int32
	IsDead bool

	// 蓄力状态
	IsCharging    bool
	ChargeStartTs int64 // 服务端时间
	FacingAngle   int32

	// 等待室状态
	IsReady bool

	// 输入缓冲区 (Sub-tick)
	InputQueue []*pb.C2SInput

	// 延迟补偿
	TargetTick        int64 // 客户端期望执行的目标tick
	LastProcessedTick int64 // 已处理的最后tick
}

func NewPlayer(uid int64, name string, conn *WebSocketConn) *Player {
	return &Player{
		UID:        uid,
		Username:   name,
		Conn:       conn,
		HP:         100,
		MaxHP:      100,
		Speed:      10.0, // 配置读取
		InputQueue: make([]*pb.C2SInput, 0),
	}
}
