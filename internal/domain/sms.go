package domain

type Sms struct {
	MessageId  string `json:"message_id"`
	CustomerId int    `json:"customer_id"`
	Priority   int    `json:"priority"`
	To         string `json:"to"`
	Message    string `json:"message"`
	CreatedAt  string `json:"created_at"`
}
