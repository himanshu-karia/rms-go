package secondary

import (
	"context"
	"encoding/json"
	"fmt"
	"ingestion-go/internal/core/ports"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(url string) *RedisStore {
	url = strings.TrimSpace(url)
	if url == "" {
		url = "redis://localhost:6379/0"
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		// Allow passing plain host:port (e.g. "localhost:6380")
		if _, _, splitErr := net.SplitHostPort(url); splitErr == nil {
			opt = &redis.Options{Addr: url}
		} else {
			opt = &redis.Options{Addr: "localhost:6379"}
		}
	}
	if opt == nil {
		opt = &redis.Options{Addr: "localhost:6379"}
	}
	return &RedisStore{
		client: redis.NewClient(opt),
	}
}

// Ensure implementation
var _ ports.StateStore = (*RedisStore)(nil)

func (r *RedisStore) AcquireLock(key string, ttlSeconds int) (bool, error) {
	ctx := context.Background()
	fullKey := "lock:" + key
	// SET key val NX EX ttl
	success, err := r.client.SetNX(ctx, fullKey, "1", time.Duration(ttlSeconds)*time.Second).Result()
	return success, err
}

func (r *RedisStore) PushPacket(deviceId string, packet interface{}) error {
	ctx := context.Background()
	key := "hot:" + deviceId

	val, _ := json.Marshal(packet)

	// Pipeline: LPUSH + LTRIM
	pipe := r.client.Pipeline()
	pipe.LPush(ctx, key, val)
	pipe.LTrim(ctx, key, 0, 49) // Keep 50
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisStore) GetPackets(deviceId string) ([]interface{}, error) {
	ctx := context.Background()
	key := "hot:" + deviceId

	vals, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var packets []interface{}
	for _, v := range vals {
		var p interface{}
		json.Unmarshal([]byte(v), &p)
		packets = append(packets, p)
	}
	return packets, nil
}

func (r *RedisStore) GetProjectConfig(projectId string) (interface{}, bool) {
	ctx := context.Background()
	key := "config:project:" + projectId
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false
	}
	var config interface{}
	json.Unmarshal([]byte(val), &config)
	return config, true
}

func (r *RedisStore) SetProjectConfig(projectId string, config interface{}) error {
	ctx := context.Background()
	key := "config:project:" + projectId
	val, _ := json.Marshal(config)
	return r.client.Set(ctx, key, val, 10*time.Minute).Err()
}

func (r *RedisStore) GetDeviceShadow(deviceId string) (map[string]interface{}, error) {
	ctx := context.Background()
	key := "shadow:" + deviceId
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var shadow map[string]interface{}
	json.Unmarshal([]byte(val), &shadow)
	return shadow, nil
}

func (s *RedisStore) SetRaw(key string, val string, ttl time.Duration) error {
	ctx := context.Background()
	return s.client.Set(ctx, key, val, ttl).Err()
}

func (s *RedisStore) GetRaw(key string) (string, bool, error) {
	ctx := context.Background()
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (s *RedisStore) Delete(key string) error {
	ctx := context.Background()
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStore) PushDeadLetter(queue string, val string, maxLen int, ttl time.Duration) error {
	ctx := context.Background()
	if strings.TrimSpace(queue) == "" {
		queue = "ingest:deadletter"
	}
	if maxLen <= 0 {
		maxLen = 10000
	}
	pipe := s.client.Pipeline()
	pipe.LPush(ctx, queue, val)
	pipe.LTrim(ctx, queue, 0, int64(maxLen-1))
	if ttl > 0 {
		pipe.Expire(ctx, queue, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) IncrementCounter(key string, ttl time.Duration) (int64, error) {
	ctx := context.Background()
	if strings.TrimSpace(key) == "" {
		key = "metrics:counter:default"
	}
	pipe := s.client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func (s *RedisStore) PopDeadLetters(queue string, count int) ([]string, error) {
	ctx := context.Background()
	if strings.TrimSpace(queue) == "" {
		queue = "ingest:deadletter"
	}
	if count <= 0 {
		count = 1
	}
	items := make([]string, 0, count)
	for i := 0; i < count; i++ {
		v, err := s.client.RPop(ctx, queue).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return items, err
		}
		items = append(items, v)
	}
	return items, nil
}

func (s *RedisStore) ListLength(key string) (int64, error) {
	ctx := context.Background()
	return s.client.LLen(ctx, key).Result()
}

// GetConfigBundle returns the merged project bundle if present (config:bundle:{projectId}).
func (s *RedisStore) GetConfigBundle(projectId string) (map[string]interface{}, bool) {
	if strings.TrimSpace(projectId) == "" {
		return nil, false
	}
	key := fmt.Sprintf("config:bundle:%s", projectId)
	if raw, ok, err := s.GetRaw(key); err == nil && ok {
		if raw == "" || raw == "null" {
			return nil, false
		}
		var bundle map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &bundle); err == nil {
			if len(bundle) == 0 {
				return nil, false
			}
			return bundle, true
		}
	}
	return nil, false
}
