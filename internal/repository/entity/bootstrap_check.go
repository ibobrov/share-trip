package entity

import (
	"time"

	"github.com/google/uuid"
)

type BootstrapCheck struct {
	ID        uuid.UUID
	CreatedAt time.Time
}
