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

	redisScanOK  = metrics.NewCounter(`redis_operations_total{operation="scan",status="ok"}`)
	redisScanErr = metrics.NewCounter(`redis_operations_total{operation="scan",status="error"}`)
	redisScanDur = metrics.NewHistogram(`redis_operation_duration_seconds{operation="scan"}`)
)

type postData struct {
	PostID            string    `json:"post_id"`
	ChannelID         string    `json:"channel_id"`
	Fingerprint       string    `json:"fingerprint"`
	AlertName         string    `json:"alert_name"`
	Severity          string    `json:"severity"`
	FiringStartTime   time.Time `json:"firing_start_time"`
	CreatedAt         time.Time `json:"created_at"`
	LastUpdated       time.Time `json:"last_updated"`
	LastKnownAssignee string    `json:"last_known_assignee,omitempty"`
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
		PostID:            p.PostID(),
		ChannelID:         p.ChannelID(),
		Fingerprint:       p.Fingerprint().Value(),
		AlertName:         p.AlertName(),
		Severity:          p.Severity().String(),
		FiringStartTime:   p.FiringStartTime(),
		CreatedAt:         p.CreatedAt(),
		LastUpdated:       p.LastUpdated(),
		LastKnownAssignee: p.LastKnownAssignee(),
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
		data.FiringStartTime,
		data.CreatedAt,
		data.LastUpdated,
		data.LastKnownAssignee,
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

func (r *PostRepository) FindAllActive(ctx context.Context) ([]*post.Post, error) {
	start := time.Now()
	pattern := keyPrefix + "*"

	// First, collect all keys using SCAN
	var allKeys []string
	var cursor uint64

	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			duration := time.Since(start).Milliseconds()
			r.logger.Error("Redis SCAN failed",
				logger.RedisFieldsWithError("scan", pattern, duration, err.Error()),
			)
			redisScanErr.Inc()
			return nil, fmt.Errorf("redis scan: %w", err)
		}

		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(allKeys) == 0 {
		duration := time.Since(start).Milliseconds()
		r.logger.Debug("Redis SCAN completed (no keys)",
			logger.RedisFields("scan", pattern, duration),
		)
		redisScanOK.Inc()
		redisScanDur.Update(float64(duration) / 1000)
		return nil, nil
	}

	// Batch fetch all values using MGET
	results, err := r.client.MGet(ctx, allKeys...).Result()
	if err != nil {
		duration := time.Since(start).Milliseconds()
		r.logger.Error("Redis MGET failed",
			logger.RedisFieldsWithError("mget", pattern, duration, err.Error()),
		)
		redisScanErr.Inc()
		return nil, fmt.Errorf("redis mget: %w", err)
	}

	posts := make([]*post.Post, 0, len(results))
	for i, result := range results {
		if result == nil {
			continue
		}

		strResult, ok := result.(string)
		if !ok {
			r.logger.Warn("Unexpected result type during MGET",
				slog.String("key", allKeys[i]),
			)
			continue
		}

		var data postData
		if err := json.Unmarshal([]byte(strResult), &data); err != nil {
			r.logger.Warn("Failed to unmarshal post data during scan",
				slog.String("key", allKeys[i]),
				slog.String("error", err.Error()),
			)
			continue
		}

		p := post.RestorePost(
			data.PostID,
			data.ChannelID,
			alert.RestoreFingerprint(data.Fingerprint),
			data.AlertName,
			alert.RestoreSeverity(data.Severity),
			data.FiringStartTime,
			data.CreatedAt,
			data.LastUpdated,
			data.LastKnownAssignee,
		)
		posts = append(posts, p)
	}

	duration := time.Since(start).Milliseconds()
	r.logger.Debug("Redis SCAN+MGET completed",
		logger.RedisFields("scan", pattern, duration),
		slog.Int("count", len(posts)),
	)
	redisScanOK.Inc()
	redisScanDur.Update(float64(duration) / 1000)

	return posts, nil
}

func (r *PostRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
