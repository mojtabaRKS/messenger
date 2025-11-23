package entity

import "time"

type KafkaDlq struct {
	Topic         string
	Key           string
	Payload       []byte
	AttemptCount  int
	Priority      int
	LastAttemptAt time.Time
}

func (KafkaDlq) TableName() string {
	return "kafka_dlq"
}
