package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/redis/go-redis/v9"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

const (
	keyPrefix = "kmbridge:alert:"
	ttl       = 7 * 24 * time.Hour
)

var (
	redisSetOK  = metrics.NewCounter(`redis_operations_total{operation="set",status="ok"}`)
	redisSetErr = metrics.NewCounter(`redis_operations_total{operation="set",status="error"}`)
	redisSetDur = metrics.NewHistogram(`redis_operation_duration_seconds{operation="set"}`)

	redisGetOK   = metrics.NewCounter(`redis_operations_total{operation="get",status="ok"}`)
	redisGetErr  = metrics.NewCounter(`redis_operations_total{operation="get",status="error"}`)
	redisGetMiss = metrics.NewCounter(`redis_operations_total{operation="get",status="miss"}`)
	redisGetDur  = metrics.NewHistogram(`redis_operation_duration_seconds{operation="get"}`)

	redisDelOK  = metrics.NewCounter(`redis_operations_total{operation="del",status="ok"}`)
	redisDelErr = metrics.NewCounter(`redis_operations_total{operation="del",status="error"}`)
)

type postData struct {
	PostID      string    `json:"post_id"`
	ChannelID   string    `json:"channel_id"`
	Fingerprint string    `json:"fingerprint"`
	AlertName   string    `json:"alert_name"`
	Severity    string    `json:"severity"`
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
}

type PostRepository struct {
	client *redis.Client
	logger *slog.Logger
}

func NewPostRepository(client *redis.Client, logger *slog.Logger) *PostRepository {
	return &PostRepository{
		client: client,
		logger: logger,
	}
}

func (r *PostRepository) Save(ctx context.Context, fingerprint alert.Fingerprint, p *post.Post) error {
	key := keyPrefix + fingerprint.Value()
	start := time.Now()

	data := postData{
		PostID:      p.PostID(),
		ChannelID:   p.ChannelID(),
		Fingerprint: p.Fingerprint().Value(),
		AlertName:   p.AlertName(),
		Severity:    p.Severity().String(),
		CreatedAt:   p.CreatedAt(),
		LastUpdated: p.LastUpdated(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal post data: %w", err)
	}

	if err := r.client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		duration := time.Since(start).Milliseconds()
		r.logger.Error("Redis SET failed",
			logger.RedisFieldsWithError("set", key, duration, err.Error()),
		)
		redisSetErr.Inc()
		return fmt.Errorf("redis set: %w", err)
	}

	duration := time.Since(start).Milliseconds()
	r.logger.Debug("Redis SET completed",
		logger.RedisFields("set", key, duration),
	)
	redisSetOK.Inc()
	redisSetDur.Update(float64(duration) / 1000)

	return nil
}

func (r *PostRepository) FindByFingerprint(ctx context.Context, fingerprint alert.Fingerprint) (*post.Post, error) {
	key := keyPrefix + fingerprint.Value()
	start := time.Now()

	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		duration := time.Since(start).Milliseconds()
		if errors.Is(err, redis.Nil) {
			r.logger.Debug("Redis GET miss",
				logger.RedisFields("get", key, duration),
			)
			redisGetMiss.Inc()
			return nil, post.ErrNotFound
		}
		r.logger.Error("Redis GET failed",
			logger.RedisFieldsWithError("get", key, duration, err.Error()),
		)
		redisGetErr.Inc()
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var data postData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, fmt.Errorf("unmarshal post data: %w", err)
	}

	duration := time.Since(start).Milliseconds()
	r.logger.Debug("Redis GET completed",
		logger.RedisFields("get", key, duration),
	)
	redisGetOK.Inc()
	redisGetDur.Update(float64(duration) / 1000)

	return post.RestorePost(
		data.PostID,
		data.ChannelID,
		alert.RestoreFingerprint(data.Fingerprint),
		data.AlertName,
		alert.RestoreSeverity(data.Severity),
		data.CreatedAt,
		data.LastUpdated,
	), nil
}

func (r *PostRepository) Delete(ctx context.Context, fingerprint alert.Fingerprint) error {
	key := keyPrefix + fingerprint.Value()
	start := time.Now()

	if err := r.client.Del(ctx, key).Err(); err != nil {
		duration := time.Since(start).Milliseconds()
		r.logger.Error("Redis DEL failed",
			logger.RedisFieldsWithError("del", key, duration, err.Error()),
		)
		redisDelErr.Inc()
		return fmt.Errorf("redis del: %w", err)
	}

	duration := time.Since(start).Milliseconds()
	r.logger.Debug("Redis DEL completed",
		logger.RedisFields("del", key, duration),
	)
	redisDelOK.Inc()

	return nil
}

func (r *PostRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
