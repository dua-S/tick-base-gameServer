package model

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"type:varchar(32);uniqueIndex;not null"`
	Password string `gorm:"type:varchar(100);not null"` // 加密后的
}

type MatchHistory struct {
	gorm.Model
	UserID    uint   `gorm:"index;not null"`
	MatchID   string `gorm:"type:varchar(64);index"` // 比赛UUID
	IsWinner  bool
	Kills     int
	Timestamp int64
}

// 初始化 DB
func InitDB(dsn string) (*gorm.DB, error) {
	// 这里仅定义结构，实际连接逻辑在 main 或 dao 层
	return nil, nil
}
