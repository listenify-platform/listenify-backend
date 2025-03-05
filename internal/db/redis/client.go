// Package redis provides Redis database connectivity and operations.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"norelock.dev/listenify/backend/internal/config"
	"norelock.dev/listenify/backend/internal/utils"
)

// Client wraps the Redis client with app-specific functionality
type Client struct {
	client *redis.Client
	logger *utils.Logger
}

// NewClient creates a new Redis client
func NewClient(cfg *config.Config, logger *utils.Logger) (*Client, error) {
	// If no logger is provided, use the global logger
	if logger == nil {
		logger = utils.GetLogger()
	}

	// Create Redis client options
	opts := &redis.Options{
		Addr:         cfg.Database.Redis.Addresses[0], // Use the first address in the list
		Username:     cfg.Database.Redis.Username,
		Password:     cfg.Database.Redis.Password,
		DB:           cfg.Database.Redis.Database,
		MaxRetries:   cfg.Database.Redis.MaxRetries,
		PoolSize:     cfg.Database.Redis.PoolSize,
		MinIdleConns: cfg.Database.Redis.MinIdleConns,
		DialTimeout:  cfg.Database.Redis.DialTimeout,
		ReadTimeout:  cfg.Database.Redis.ReadTimeout,
		WriteTimeout: cfg.Database.Redis.WriteTimeout,
		IdleTimeout:  cfg.Database.Redis.IdleTimeout,
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to connect to Redis", err, "addr", opts.Addr)
		return nil, err
	}

	logger.Info("Connected to Redis", "addr", opts.Addr, "db", opts.DB)

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	err := c.client.Close()
	if err != nil {
		c.logger.Error("Failed to close Redis connection", err)
		return err
	}
	c.logger.Info("Closed Redis connection")
	return nil
}

// Client returns the underlying Redis client
func (c *Client) Client() *redis.Client {
	return c.client
}

// Ping pings the Redis server
func (c *Client) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		c.logger.Error("Failed to ping Redis", err)
		return err
	}
	return nil
}

// Get gets a value from Redis
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Key does not exist
			return "", nil
		}
		c.logger.Error("Failed to get value from Redis", err, "key", key)
		return "", err
	}
	return value, nil
}

// GetObject gets an object from Redis and unmarshals it
func (c *Client) GetObject(ctx context.Context, key string, dest any) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}

	if data == "" {
		return redis.Nil
	}

	return json.Unmarshal([]byte(data), dest)
}

// Set sets a value in Redis with an optional expiration
func (c *Client) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	if err := c.client.Set(ctx, key, value, expiration).Err(); err != nil {
		c.logger.Error("Failed to set value in Redis", err, "key", key)
		return err
	}
	return nil
}

// SetObject sets an object in Redis by marshaling it to JSON
func (c *Client) SetObject(ctx context.Context, key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		c.logger.Error("Failed to marshal object for Redis", err, "key", key)
		return err
	}

	return c.Set(ctx, key, string(data), expiration)
}

// Del deletes a key from Redis
func (c *Client) Del(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Error("Failed to delete key from Redis", err, "key", key)
		return err
	}
	return nil
}

// DelKeys deletes multiple keys from Redis
func (c *Client) DelKeys(ctx context.Context, pattern string) error {
	keys, err := c.Keys(ctx, pattern)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Error("Failed to delete keys from Redis", err, "pattern", pattern)
		return err
	}

	return nil
}

// Keys gets all keys matching a pattern
func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		c.logger.Error("Failed to get keys from Redis", err, "pattern", pattern)
		return nil, err
	}
	return keys, nil
}

// Exists checks if a key exists in Redis
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to check if key exists in Redis", err, "key", key)
		return false, err
	}
	return exists > 0, nil
}

// Expire sets an expiration on a key
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if err := c.client.Expire(ctx, key, expiration).Err(); err != nil {
		c.logger.Error("Failed to set expiration on key", err, "key", key)
		return err
	}
	return nil
}

// TTL gets the TTL of a key
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get TTL of key", err, "key", key)
		return 0, err
	}
	return ttl, nil
}

// Incr increments a key
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	value, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to increment key", err, "key", key)
		return 0, err
	}
	return value, nil
}

// IncrBy increments a key by a specific amount
func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	result, err := c.client.IncrBy(ctx, key, value).Result()
	if err != nil {
		c.logger.Error("Failed to increment key by value", err, "key", key, "value", value)
		return 0, err
	}
	return result, nil
}

// Decr decrements a key
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	value, err := c.client.Decr(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to decrement key", err, "key", key)
		return 0, err
	}
	return value, nil
}

// DecrBy decrements a key by a specific amount
func (c *Client) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	result, err := c.client.DecrBy(ctx, key, value).Result()
	if err != nil {
		c.logger.Error("Failed to decrement key by value", err, "key", key, "value", value)
		return 0, err
	}
	return result, nil
}

// HSet sets a hash field
func (c *Client) HSet(ctx context.Context, key, field string, value any) error {
	if err := c.client.HSet(ctx, key, field, value).Err(); err != nil {
		c.logger.Error("Failed to set hash field", err, "key", key, "field", field)
		return err
	}
	return nil
}

// HSetObject sets a hash field with a JSON-marshaled object
func (c *Client) HSetObject(ctx context.Context, key, field string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		c.logger.Error("Failed to marshal object for hash field", err, "key", key, "field", field)
		return err
	}

	return c.HSet(ctx, key, field, string(data))
}

// HGet gets a hash field
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	value, err := c.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
			// Field does not exist
			return "", nil
		}
		c.logger.Error("Failed to get hash field", err, "key", key, "field", field)
		return "", err
	}
	return value, nil
}

// HGetObject gets a hash field and unmarshals it
func (c *Client) HGetObject(ctx context.Context, key, field string, dest any) error {
	data, err := c.HGet(ctx, key, field)
	if err != nil {
		return err
	}

	if data == "" {
		return redis.Nil
	}

	return json.Unmarshal([]byte(data), dest)
}

// HGetAll gets all fields in a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	values, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get all hash fields", err, "key", key)
		return nil, err
	}
	return values, nil
}

// HDel deletes a hash field
func (c *Client) HDel(ctx context.Context, key, field string) error {
	if err := c.client.HDel(ctx, key, field).Err(); err != nil {
		c.logger.Error("Failed to delete hash field", err, "key", key, "field", field)
		return err
	}
	return nil
}

// HExists checks if a hash field exists
func (c *Client) HExists(ctx context.Context, key, field string) (bool, error) {
	exists, err := c.client.HExists(ctx, key, field).Result()
	if err != nil {
		c.logger.Error("Failed to check if hash field exists", err, "key", key, "field", field)
		return false, err
	}
	return exists, nil
}

// HLen gets the number of fields in a hash
func (c *Client) HLen(ctx context.Context, key string) (int64, error) {
	length, err := c.client.HLen(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get hash length", err, "key", key)
		return 0, err
	}
	return length, nil
}

// LPush prepends an element to a list
func (c *Client) LPush(ctx context.Context, key string, value any) error {
	if err := c.client.LPush(ctx, key, value).Err(); err != nil {
		c.logger.Error("Failed to push element to list", err, "key", key)
		return err
	}
	return nil
}

// RPush appends an element to a list
func (c *Client) RPush(ctx context.Context, key string, value any) error {
	if err := c.client.RPush(ctx, key, value).Err(); err != nil {
		c.logger.Error("Failed to push element to list", err, "key", key)
		return err
	}
	return nil
}

// LPop removes and returns the first element of a list
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	value, err := c.client.LPop(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// List is empty
			return "", nil
		}
		c.logger.Error("Failed to pop element from list", err, "key", key)
		return "", err
	}
	return value, nil
}

// RPop removes and returns the last element of a list
func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	value, err := c.client.RPop(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// List is empty
			return "", nil
		}
		c.logger.Error("Failed to pop element from list", err, "key", key)
		return "", err
	}
	return value, nil
}

// LRange gets a range of elements from a list
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	values, err := c.client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		c.logger.Error("Failed to get range from list", err, "key", key)
		return nil, err
	}
	return values, nil
}

// LLen gets the length of a list
func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	length, err := c.client.LLen(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get list length", err, "key", key)
		return 0, err
	}
	return length, nil
}

// LRem removes elements from a list
func (c *Client) LRem(ctx context.Context, key string, count int64, value any) error {
	if err := c.client.LRem(ctx, key, count, value).Err(); err != nil {
		c.logger.Error("Failed to remove elements from list", err, "key", key)
		return err
	}
	return nil
}

// SAdd adds a member to a set
func (c *Client) SAdd(ctx context.Context, key string, members ...any) error {
	if err := c.client.SAdd(ctx, key, members...).Err(); err != nil {
		c.logger.Error("Failed to add members to set", err, "key", key)
		return err
	}
	return nil
}

// SMembers gets all members of a set
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	members, err := c.client.SMembers(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get set members", err, "key", key)
		return nil, err
	}
	return members, nil
}

// SIsMember checks if a value is a member of a set
func (c *Client) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	isMember, err := c.client.SIsMember(ctx, key, member).Result()
	if err != nil {
		c.logger.Error("Failed to check set membership", err, "key", key)
		return false, err
	}
	return isMember, nil
}

// SRem removes a member from a set
func (c *Client) SRem(ctx context.Context, key string, members ...any) error {
	if err := c.client.SRem(ctx, key, members...).Err(); err != nil {
		c.logger.Error("Failed to remove members from set", err, "key", key)
		return err
	}
	return nil
}

// SCard gets the number of members in a set
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	count, err := c.client.SCard(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get set cardinality", err, "key", key)
		return 0, err
	}
	return count, nil
}

// ZAdd adds a member to a sorted set
func (c *Client) ZAdd(ctx context.Context, key string, score float64, member string) error {
	z := redis.Z{
		Score:  score,
		Member: member,
	}
	if err := c.client.ZAdd(ctx, key, &z).Err(); err != nil {
		c.logger.Error("Failed to add member to sorted set", err, "key", key, "member", member)
		return err
	}
	return nil
}

// ZRange gets a range of members from a sorted set by index
func (c *Client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	members, err := c.client.ZRange(ctx, key, start, stop).Result()
	if err != nil {
		c.logger.Error("Failed to get range from sorted set", err, "key", key)
		return nil, err
	}
	return members, nil
}

// ZRangeWithScores gets a range of members with scores from a sorted set by index
func (c *Client) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	members, err := c.client.ZRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		c.logger.Error("Failed to get range with scores from sorted set", err, "key", key)
		return nil, err
	}
	return members, nil
}

// ZRank gets the rank of a member in a sorted set
func (c *Client) ZRank(ctx context.Context, key, member string) (int64, error) {
	rank, err := c.client.ZRank(ctx, key, member).Result()
	if err != nil {
		if err == redis.Nil {
			// Member does not exist
			return -1, nil
		}
		c.logger.Error("Failed to get rank in sorted set", err, "key", key, "member", member)
		return 0, err
	}
	return rank, nil
}

// ZRem removes members from a sorted set
func (c *Client) ZRem(ctx context.Context, key string, members ...any) error {
	if err := c.client.ZRem(ctx, key, members...).Err(); err != nil {
		c.logger.Error("Failed to remove members from sorted set", err, "key", key)
		return err
	}
	return nil
}

// ZCard gets the number of members in a sorted set
func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	count, err := c.client.ZCard(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to get sorted set cardinality", err, "key", key)
		return 0, err
	}
	return count, nil
}

// Pipeline creates a Redis pipeline
func (c *Client) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

// TxPipeline creates a Redis transaction pipeline
func (c *Client) TxPipeline() redis.Pipeliner {
	return c.client.TxPipeline()
}

// Publish publishes a message to a channel
func (c *Client) Publish(ctx context.Context, channel string, message any) error {
	if err := c.client.Publish(ctx, channel, message).Err(); err != nil {
		c.logger.Error("Failed to publish message", err, "channel", channel)
		return err
	}
	return nil
}

// Subscribe subscribes to channels
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// Logger returns the logger used by the client
func (c *Client) Logger() *utils.Logger {
	return c.logger
}

// FormatKey creates a namespaced Redis key
func FormatKey(namespace, key string) string {
	return fmt.Sprintf("%s:%s", namespace, key)
}
