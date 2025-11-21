package provider

import "arvan/message-gateway/internal/domain"

// SMSProvider defines the abstraction for sending SMS messages.
type SMSProvider interface {
	Send(job domain.Job) error
}

// StubProvider simulates latency and occasional failures for demo/testing.
type StubProvider struct{}

// NewStubProvider constructs a stub provider.
func NewStubProvider() SMSProvider {
	return &StubProvider{}
}
