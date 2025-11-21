package provider

import (
	"arvan/message-gateway/internal/domain"
	"fmt"
)

func (s *StubProvider) Send(job domain.Job) error {
	// Simulate latency 50-200ms
	//lat := 50 + rand.Intn(151)
	//time.Sleep(time.Duration(lat) * time.Millisecond)

	// 10% failure
	//if rand.Float32() < 0.1 {
	//	return fmt.Errorf("provider: simulated failure for job %s", job.ID)
	//}
	//
	//// 20% proccess may take longer than usual
	//if rand.Float32() < 0.2 {
	//	time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
	//	return nil
	//}

	// print success message
	fmt.Print("Sms sent successfully")
	return nil
}
