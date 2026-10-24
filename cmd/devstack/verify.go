package main

import (
	"fmt"
	"net/http"
	"time"
)

func (s *stack) verify() error {
	client := &http.Client{Timeout: 3 * time.Second}
	for _, check := range []struct {
		url    string
		status int
	}{
		{"http://localhost:38080/metrics", 200},
		{"http://localhost:38080/api/trip/00000000-0000-4000-8000-000000000001", 401},
		{"http://localhost:38082/api/contract.yaml", 200},
		{"http://localhost:38087/realms/sharetrip/.well-known/openid-configuration", 200},
		{"http://localhost:39090/-/ready", 200},
		{"http://localhost:33000/api/health", 200},
	} {
		deadline := time.Now().Add(time.Minute)
		for {
			req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, check.url, nil)
			if err != nil {
				return err
			}
			response, err := client.Do(req)
			status := 0
			if err == nil {
				status = response.StatusCode
				_ = response.Body.Close()
			}
			if status == check.status {
				fmt.Println("Checked", check.url, status)
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("%s: expected HTTP %d, got %d; check LoadBalancer services and pod logs", check.url, check.status, status)
			}
			select {
			case <-s.ctx.Done():
				return s.ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil
}
