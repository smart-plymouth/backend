package taskqueue

import (
	"github.com/hibiken/asynq"
)

type Client struct {
	*asynq.Client
}

func NewClient(redisURL string) *Client {
	// Parse redis URL to get address
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		// Fallback to localhost
		opt = asynq.RedisClientOpt{Addr: "localhost:6379"}
	}
	return &Client{Client: asynq.NewClient(opt)}
}
