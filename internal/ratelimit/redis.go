package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client   *redis.Client
	cooldown time.Duration
}

func NewRedisLimiter(client *redis.Client, cooldown time.Duration) *RedisLimiter {
	return &RedisLimiter{
		client:   client,
		cooldown: cooldown,
	}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	acquired, err := l.client.SetNX(
		ctx,
		key,
		1,
		l.cooldown,
	).Result()
	if err != nil {
		return false, 0, ErrInternalServer
	}

	if acquired {
		return true, 0, nil
	}

	ttl, err := l.client.TTL(ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("get rate limit ttl: %w", err)
	}

	return false, ttl, nil
}
