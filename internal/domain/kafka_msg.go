package domain

type KafkaMessage struct {
	Key      string
	Payload  []byte
	Topic    string
	Attempts int
}
