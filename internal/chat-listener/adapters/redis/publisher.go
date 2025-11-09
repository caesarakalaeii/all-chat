package redis

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// Publisher implements the Publisher interface using Redis pub/sub
type Publisher struct {
	client *redis.Client
}

// NewPublisher creates a new Redis publisher
func NewPublisher(client *redis.Client) *Publisher {
	return &Publisher{
		client: client,
	}
}

// Publish sends a message to a Redis channel
func (p *Publisher) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return p.client.Publish(ctx, channel, data).Err()
}
