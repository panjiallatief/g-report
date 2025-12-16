package redis

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var ctx = context.Background()

// Init initializes the Redis client connection
func Init() error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379" // Default
	}

	Client = redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: "", // No password by default
		DB:       0,  // Default DB
	})

	// Test connection
	_, err := Client.Ping(ctx).Result()
	if err != nil {
		log.Printf("[Redis] ❌ Failed to connect: %v", err)
		return err
	}

	log.Println("[Redis] ✅ Connected to Redis at", redisURL)
	return nil
}

// === CACHE FUNCTIONS ===

// Set stores a value in cache with TTL
func Set(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves a value from cache and unmarshals into dest
func Get(key string, dest interface{}) error {
	val, err := Client.Get(ctx, key).Result()
	if err != nil {
		return err // redis.Nil if key doesn't exist
	}
	return json.Unmarshal([]byte(val), dest)
}

// Delete removes a key from cache
func Delete(keys ...string) error {
	return Client.Del(ctx, keys...).Err()
}

// DeletePattern removes all keys matching a pattern (e.g., "cache:bigbook:*")
func DeletePattern(pattern string) error {
	iter := Client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		Client.Del(ctx, iter.Val())
	}
	return iter.Err()
}

// === PUB/SUB FUNCTIONS ===

// Publish sends a message to a channel
func Publish(channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return Client.Publish(ctx, channel, data).Err()
}

// Subscribe returns a PubSub subscription for a channel
func Subscribe(channel string) *redis.PubSub {
	return Client.Subscribe(ctx, channel)
}

// IsConnected checks if Redis is available
func IsConnected() bool {
	if Client == nil {
		return false
	}
	_, err := Client.Ping(ctx).Result()
	return err == nil
}
