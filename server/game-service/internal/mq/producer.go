package mq

import (
	"encoding/json"
	"log"
	"mygame/server/game-service/pkg/config"

	"github.com/streadway/amqp"
)

var Channel *amqp.Channel

func InitMQ() {
	conn, err := amqp.Dial(config.AppConfig.MQ.Url)
	if err != nil {
		log.Fatalf("MQ connect failed: %v", err)
	}

	Channel, err = conn.Channel()
	if err != nil {
		log.Fatalf("MQ channel failed: %v", err)
	}

	// 声明队列
	_, err = Channel.QueueDeclare(
		config.AppConfig.MQ.QueueName,
		true, false, false, false, nil,
	)
	if err != nil {
		log.Fatalf("MQ queue declare failed: %v", err)
	}
}

func PublishGameResult(result interface{}) {
	body, _ := json.Marshal(result)
	err := Channel.Publish(
		"",
		config.AppConfig.MQ.QueueName,
		false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		log.Printf("Failed to publish result: %v", err)
	}
}
