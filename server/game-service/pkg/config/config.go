package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MQ     MQConfig     `mapstructure:"mq"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Game   GameConfig   `mapstructure:"game"`
}

type ServerConfig struct {
	Port     int `mapstructure:"port"`
	GrpcPort int `mapstructure:"grpc_port"`
	TickRate int `mapstructure:"tick_rate"`
}

type MQConfig struct {
	Url       string `mapstructure:"url"`
	QueueName string `mapstructure:"queue_name"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type GameConfig struct {
	MapSize     float64 `mapstructure:"map_size"`
	PlayerSpeed float64 `mapstructure:"player_speed"`
	ViewRadius  float64 `mapstructure:"view_radius"`
}

var AppConfig *Config

func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}
	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}
}
