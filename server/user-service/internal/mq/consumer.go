package mq

import (
	"encoding/json"
	"log"
	"mygame/server/user-service/internal/dao"
	"mygame/server/user-service/model"
	"mygame/server/user-service/pkg/config"

	"github.com/streadway/amqp"
)

var Conn *amqp.Connection
var Channel *amqp.Channel

type GameResult struct {
	MatchID   string `json:"match_id"`
	Winner    int64  `json:"winner"`
	Timestamp int64  `json:"timestamp"`
}

func InitMQ() {
	var err error
	Conn, err = amqp.Dial(config.AppConfig.MQ.Url)
	if err != nil {
		log.Fatalf("MQ connect failed: %v", err)
	}

	Channel, err = Conn.Channel()
	if err != nil {
		log.Fatalf("MQ channel failed: %v", err)
	}

	_, err = Channel.QueueDeclare(
		config.AppConfig.MQ.QueueName,
		true, false, false, false, nil,
	)
	if err != nil {
		log.Fatalf("MQ queue declare failed: %v", err)
	}
}

func StartConsumer() {
	msgs, err := Channel.Consume(
		config.AppConfig.MQ.QueueName,
		"",
		false, // auto-ack
		false, false, false, nil,
	)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	log.Printf("MQ Consumer started, waiting for messages on queue: %s", config.AppConfig.MQ.QueueName)

	for msg := range msgs {
		var result GameResult
		if err := json.Unmarshal(msg.Body, &result); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			msg.Nack(false, false)
			continue
		}

		if err := saveGameResult(&result); err != nil {
			log.Printf("Failed to save game result: %v", err)
			msg.Nack(false, true) // requeue
			continue
		}

		msg.Ack(false)
		log.Printf("Game result saved: match_id=%s, winner=%d", result.MatchID, result.Winner)
	}
}

func saveGameResult(result *GameResult) error {
	history := &model.MatchHistory{
		MatchID:   result.MatchID,
		UserID:    uint(result.Winner),
		IsWinner:  true,
		Kills:     0,
		Timestamp: result.Timestamp,
	}
	return dao.AddHistory(history)
}
