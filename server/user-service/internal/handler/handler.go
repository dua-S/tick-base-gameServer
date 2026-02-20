package handler

import (
	"context"
	pb "mygame/proto"
	"mygame/server/user-service/internal/dao"
	"mygame/server/user-service/internal/service"
	"mygame/server/user-service/model"
)

type UserService struct {
	pb.UnimplementedUserServiceServer
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	// 1. 密码加密
	hashedPwd, err := service.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 2. 存入 DB
	user := &model.User{
		Username: req.Username,
		Password: hashedPwd,
	}
	if err := dao.CreateUser(user); err != nil {
		return nil, err // 可能是用户名重复
	}

	return &pb.RegisterResp{Uid: int64(user.ID)}, nil
}

func (s *UserService) Login(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	// 1. 查用户
	user, err := dao.GetUserByUsername(req.Username)
	if err != nil {
		return nil, err // 用户不存在
	}

	// 2. 校验密码
	if !service.CheckPasswordHash(req.Password, user.Password) {
		return nil, err // 密码错误 (实际应返回自定义 error code)
	}

	// 3. 生成 Token
	token, err := service.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	return &pb.LoginResp{
		Token:    token,
		Uid:      int64(user.ID),
		Username: user.Username,
	}, nil
}

func (s *UserService) GetHistory(ctx context.Context, req *pb.GetHistoryReq) (*pb.GetHistoryResp, error) {
	records, err := dao.GetHistory(uint(req.Uid), int(req.Page), int(req.Limit))
	if err != nil {
		return nil, err
	}

	// 转换 Model -> Proto
	var pbHistory []*pb.MatchRecord
	for _, r := range records {
		pbHistory = append(pbHistory, &pb.MatchRecord{
			MatchId:   r.MatchID,
			IsWinner:  r.IsWinner,
			Kills:     int32(r.Kills),
			Timestamp: r.Timestamp,
		})
	}

	return &pb.GetHistoryResp{History: pbHistory}, nil
}
