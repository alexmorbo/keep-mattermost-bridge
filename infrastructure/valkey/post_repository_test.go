package valkey

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

func setupTestRedis(t *testing.T) (*PostRepository, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewPostRepository(client, logger)

	return repo, mr
}

func TestSaveAndFindByFingerprint(t *testing.T) {
	repo, _ := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-test-123")
	p := post.NewPost("post-abc", "channel-xyz", alert.RestoreFingerprint("fp-test-123"), "Test Alert", alert.RestoreSeverity("critical"), time.Now())

	err := repo.Save(ctx, fingerprint, p)
	require.NoError(t, err)

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, "post-abc", found.PostID())
	assert.Equal(t, "channel-xyz", found.ChannelID())
	assert.Equal(t, "fp-test-123", found.Fingerprint().Value())
	assert.Equal(t, "Test Alert", found.AlertName())
	assert.Equal(t, "critical", found.Severity().Value())
	assert.False(t, found.CreatedAt().IsZero())
	assert.False(t, found.LastUpdated().IsZero())
}

func TestFindByFingerprintNotFound(t *testing.T) {
	repo, _ := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("non-existent")

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.Error(t, err)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, post.ErrNotFound)
}

func TestSaveOverwritesExisting(t *testing.T) {
	repo, _ := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-overwrite")

	p1 := post.NewPost("post-1", "channel-1", alert.RestoreFingerprint("fp-overwrite"), "Alert 1", alert.RestoreSeverity("high"), time.Now())
	err := repo.Save(ctx, fingerprint, p1)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	p2 := post.NewPost("post-2", "channel-2", alert.RestoreFingerprint("fp-overwrite"), "Alert 2", alert.RestoreSeverity("critical"), time.Now())
	err = repo.Save(ctx, fingerprint, p2)
	require.NoError(t, err)

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.NoError(t, err)
	assert.Equal(t, "post-2", found.PostID())
	assert.Equal(t, "channel-2", found.ChannelID())
	assert.Equal(t, "Alert 2", found.AlertName())
	assert.Equal(t, "critical", found.Severity().Value())
}

func TestDelete(t *testing.T) {
	repo, _ := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-delete")
	p := post.NewPost("post-del", "channel-del", alert.RestoreFingerprint("fp-delete"), "Delete Test", alert.RestoreSeverity("warning"), time.Now())

	err := repo.Save(ctx, fingerprint, p)
	require.NoError(t, err)

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.NoError(t, err)
	assert.NotNil(t, found)

	err = repo.Delete(ctx, fingerprint)
	require.NoError(t, err)

	found, err = repo.FindByFingerprint(ctx, fingerprint)
	require.Error(t, err)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, post.ErrNotFound)
}

func TestPing(t *testing.T) {
	repo, mr := setupTestRedis(t)
	ctx := context.Background()

	err := repo.Ping(ctx)
	require.NoError(t, err)

	mr.Close()

	err = repo.Ping(ctx)
	require.Error(t, err)
}

func TestTTLVerification(t *testing.T) {
	repo, mr := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-ttl")
	p := post.NewPost("post-ttl", "channel-ttl", alert.RestoreFingerprint("fp-ttl"), "TTL Test", alert.RestoreSeverity("info"), time.Now())

	err := repo.Save(ctx, fingerprint, p)
	require.NoError(t, err)

	key := keyPrefix + fingerprint.Value()
	ttlDuration := mr.TTL(key)

	assert.Greater(t, ttlDuration, time.Duration(0), "TTL should be set")
	assert.LessOrEqual(t, ttlDuration, ttl, "TTL should not exceed configured value")
	assert.GreaterOrEqual(t, ttlDuration, ttl-time.Second, "TTL should be close to configured value")
}

func TestSavePreservesAllFields(t *testing.T) {
	repo, _ := setupTestRedis(t)
	ctx := context.Background()

	firingStartTime := time.Now().Add(-2 * time.Hour)
	createdTime := time.Now().Add(-1 * time.Hour)
	updatedTime := time.Now().Add(-30 * time.Minute)

	fingerprint := alert.RestoreFingerprint("fp-fields")
	p := post.RestorePost(
		"post-fields",
		"channel-fields",
		alert.RestoreFingerprint("fp-fields"),
		"Field Test Alert",
		alert.RestoreSeverity("high"),
		firingStartTime,
		createdTime,
		updatedTime,
	)

	err := repo.Save(ctx, fingerprint, p)
	require.NoError(t, err)

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.NoError(t, err)

	assert.Equal(t, "post-fields", found.PostID())
	assert.Equal(t, "channel-fields", found.ChannelID())
	assert.Equal(t, "fp-fields", found.Fingerprint().Value())
	assert.Equal(t, "Field Test Alert", found.AlertName())
	assert.Equal(t, "high", found.Severity().Value())
	assert.WithinDuration(t, createdTime, found.CreatedAt(), time.Millisecond)
	assert.WithinDuration(t, updatedTime, found.LastUpdated(), time.Millisecond)
}

func TestNewPostRepository(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	repo := NewPostRepository(client, logger)

	require.NotNil(t, repo)
	assert.NotNil(t, repo.client)
	assert.NotNil(t, repo.logger)
}

func TestSaveRedisSetError(t *testing.T) {
	repo, mr := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-error")
	p := post.NewPost("post-err", "channel-err", alert.RestoreFingerprint("fp-error"), "Error Test", alert.RestoreSeverity("critical"), time.Now())

	mr.Close()

	err := repo.Save(ctx, fingerprint, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis set")
}

func TestFindByFingerprintRedisGetError(t *testing.T) {
	repo, mr := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-get-error")

	mr.Close()

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.Error(t, err)
	assert.Nil(t, found)
	assert.Contains(t, err.Error(), "redis get")
	assert.NotErrorIs(t, err, post.ErrNotFound)
}

func TestFindByFingerprintUnmarshalError(t *testing.T) {
	repo, mr := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-corrupt")
	key := keyPrefix + fingerprint.Value()

	_ = mr.Set(key, "invalid-json-data")

	found, err := repo.FindByFingerprint(ctx, fingerprint)
	require.Error(t, err)
	assert.Nil(t, found)
	assert.Contains(t, err.Error(), "unmarshal post data")
}

func TestDeleteRedisDelError(t *testing.T) {
	repo, mr := setupTestRedis(t)
	ctx := context.Background()

	fingerprint := alert.RestoreFingerprint("fp-del-error")

	mr.Close()

	err := repo.Delete(ctx, fingerprint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis del")
}
