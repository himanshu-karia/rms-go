package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client

func InitRedis(redisUrl string) {
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		log.Fatalf("❌ Invalid Redis URL: %v", err)
	}

	Rdb = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Connected to Redis")
}

// Deduplicate checks if a message ID has been processed in the last 10 seconds.
// Returns true if the message is UNIQUE (not a duplicate).
func Deduplicate(ctx context.Context, msgID string) (bool, error) {
	return DeduplicateKey(ctx, fmt.Sprintf("dedup:%s", msgID), 10*time.Second)
}

// DeduplicateKey checks if a dedup key has been seen within TTL.
// Returns true if the key is UNIQUE (not a duplicate).
func DeduplicateKey(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return true, nil
	}
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	unique, err := Rdb.SetNX(ctx, key, "1", ttl).Result()
	return unique, err
}

// DedupTTLFromEnv returns a configurable TTL for dedup keys.
func DedupTTLFromEnv() time.Duration {
	// Default: 1 hour (cross-gateway forwarded-node dedup needs longer than a few seconds).
	seconds := 3600
	if v := strings.TrimSpace(os.Getenv("DEDUP_TTL_SECONDS")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			seconds = parsed
		}
	}
	return time.Duration(seconds) * time.Second
}

// GetDeviceMetadata retrieves device attributes (project_id, uuid, etc.) from Redis.
// Compatible with Node.js "device:imei:{imei}" JSON String cache.
func GetDeviceMetadata(ctx context.Context, imei string) (map[string]string, error) {
	key := fmt.Sprintf("device:imei:%s", imei)

	// Node.js stores this as a JSON string: {"uuid":"...", "projectId":"...", ...}
	val, err := Rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Parse JSON into map
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(val), &rawData); err != nil {
		return nil, err
	}

	// Convert to map[string]string for consistency
	meta := make(map[string]string)
	for k, v := range rawData {
		meta[k] = fmt.Sprintf("%v", v)
	}
	return meta, nil
}
