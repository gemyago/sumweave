// Package appevents provides the typed domain-event API over appdispatch.
package appevents

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
)

// Event identifies the durable topic for a typed application fact.
type Event interface {
	Topic() string
}

type messagePublisher interface {
	Publish(context.Context, appdispatch.Message) error
	PublishInTx(context.Context, *sql.Tx, appdispatch.Message) error
}

// Publisher serializes typed events onto the app-owned durable transport.
type Publisher struct {
	publisher messagePublisher
}

func NewPublisher(publisher messagePublisher) (*Publisher, error) {
	if publisher == nil {
		return nil, errors.New("message publisher is required")
	}
	return &Publisher{publisher: publisher}, nil
}

func (p *Publisher) Publish(ctx context.Context, event Event) error {
	message, err := makeMessage(event)
	if err != nil {
		return err
	}
	if err = p.publisher.Publish(ctx, message); err != nil {
		return fmt.Errorf("publish domain event %s: %w", message.Topic, err)
	}
	return nil
}

func (p *Publisher) PublishInTx(ctx context.Context, tx *sql.Tx, event Event) error {
	message, err := makeMessage(event)
	if err != nil {
		return err
	}
	if err = p.publisher.PublishInTx(ctx, tx, message); err != nil {
		return fmt.Errorf("publish domain event %s in transaction: %w", message.Topic, err)
	}
	return nil
}

func makeMessage(event Event) (appdispatch.Message, error) {
	if event == nil {
		return appdispatch.Message{}, errors.New("domain event is required")
	}
	topic := event.Topic()
	if topic == "" {
		return appdispatch.Message{}, errors.New("domain event topic is required")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return appdispatch.Message{}, fmt.Errorf("marshal domain event: %w", err)
	}
	return appdispatch.NewMessage(topic, payload), nil
}

// MakeHandler creates a raw transport handler that decodes one event type.
func MakeHandler[EventType Event](
	example EventType,
	run func(context.Context, EventType) error,
) (appdispatch.Handler, error) {
	if run == nil {
		return appdispatch.Handler{}, errors.New("event handler run func is required")
	}
	topic := example.Topic()
	if topic == "" {
		return appdispatch.Handler{}, errors.New("domain event topic is required")
	}
	return appdispatch.NewHandler(topic, func(ctx context.Context, message appdispatch.Message) error {
		var event EventType
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return fmt.Errorf("decode domain event on topic %s: %w", topic, err)
		}
		return run(ctx, event)
	})
}

// RegisterHandler binds one typed event handler to a named appdispatch router.
func RegisterHandler[EventType Event](
	router *appdispatch.Router,
	example EventType,
	run func(context.Context, EventType) error,
) error {
	if router == nil {
		return errors.New("event router is required")
	}
	handler, err := MakeHandler(example, run)
	if err != nil {
		return err
	}
	return router.Handle(handler)
}
