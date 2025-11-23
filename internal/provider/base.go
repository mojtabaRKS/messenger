package provider

import "arvan/message-gateway/internal/domain"

type SMSProvider interface {
	Send(job domain.Job) error
}

type StubProvider struct{}

func NewStubProvider() SMSProvider {
	return &StubProvider{}
}
