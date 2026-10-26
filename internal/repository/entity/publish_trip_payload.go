package entity

import "github.com/google/uuid"

type PublishTripPayload struct {
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`
	Traceparent   string `json:"traceparent,omitempty"`
	Tracestate    string `json:"tracestate,omitempty"`

	TripID    uuid.UUID `json:"trip_id"`
	ClientID  uuid.UUID `json:"client_id"`
	CompanyID string    `json:"company_id"`
}
