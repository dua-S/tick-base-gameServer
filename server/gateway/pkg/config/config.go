package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	RPC    RPCConfig    `mapstructure:"rpc"`
	JWT    JWTConfig    `mapstructure:"jwt"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type RPCConfig struct {
	UserServiceAddr  string `mapstructure:"user_service_addr"`
	MatchServiceAddr string `mapstructure:"match_service_addr"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

var AppConfig *Config

func InitConfig() {
	viper.SetConfigName("config") // 配置文件名称(无扩展名)
	viper.SetConfigType("yaml")   // 如果配置文件的名称中没有扩展名，则需要配置此项
	viper.AddConfigPath(".")      // 查找配置文件所在的路径

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
	log.Println("Config loaded successfully")
}
