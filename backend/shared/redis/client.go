package redis

import "github.com/redis/go-redis/v9"

// NewClient returns a Redis client connected to localhost
// Each service calls this to get a consistent connection
func NewClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}