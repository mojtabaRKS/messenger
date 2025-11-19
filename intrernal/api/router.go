package api

import (
	"arvan/message-gateway/intrernal/api/handler/sms"
	"arvan/message-gateway/intrernal/api/middleware"
)

// SetupAPIRoutes
// @title						SMS gateway Service
// @version         			1.0.0
// @description     			This APIs return SMS-Gateway Methods
// @Host 						localhost:8080
// @BasePath  					/
// @Schemes 					https
func (s *Server) SetupAPIRoutes(smsHandler *sms.SmsHandler) {
	r := s.engine

	v1 := r.Group("v1")
	v1.Use(middleware.HandleAuth())
	{
		v1.GET("/sms/send", smsHandler.Send)
	}
}
