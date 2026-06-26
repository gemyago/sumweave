package domain

import (
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
)

type ConnectionSecret struct {
	ID        string
	Provider  string
	Reference string
	Envelope  credentials.Envelope
	CreatedAt time.Time
	UpdatedAt time.Time
}
