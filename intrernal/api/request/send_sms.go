package request

type SendSmsRequest struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}
