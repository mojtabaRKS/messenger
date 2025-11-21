package domain

type KafkaMessage struct {
	Key     string
	Payload []byte
	Topic   string
	// onFailAttempted indicates how many times producers/workers attempted writes (optional)
	Attempts int
}
