package dao

import (
	"log"
	"mygame/server/user-service/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitMySQL(dsn string) {
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("MySQL connect failed: %v", err)
	}
	// 自动迁移表结构
	DB.AutoMigrate(&model.User{}, &model.MatchHistory{})
}

// CreateUser 创建用户
func CreateUser(u *model.User) error {
	return DB.Create(u).Error
}

// GetUserByUsername 根据用户名查询
func GetUserByUsername(username string) (*model.User, error) {
	var user model.User
	err := DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

// GetHistory 分页查询战绩
func GetHistory(uid uint, page, limit int) ([]model.MatchHistory, error) {
	var history []model.MatchHistory
	offset := (page - 1) * limit
	err := DB.Where("user_id = ?", uid).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&history).Error
	return history, err
}

// AddHistory (用于 MQ 消费后写入)
func AddHistory(h *model.MatchHistory) error {
	return DB.Create(h).Error
}
