package mattermost

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

func TestCreatePostSuccess(t *testing.T) {
	var capturedRequest createPostRequest
	var capturedToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/posts", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		authHeader := r.Header.Get("Authorization")
		require.NotEmpty(t, authHeader)
		assert.Contains(t, authHeader, "Bearer ")
		capturedToken = authHeader[len("Bearer "):]

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &capturedRequest)
		require.NoError(t, err)

		w.WriteHeader(http.StatusCreated)
		response := createPostResponse{ID: "post-123"}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token-456", logger)

	attachment := post.Attachment{
		Color: "#FF0000",
		Title: "Test Alert",
		Text:  "Test message",
	}

	postID, err := client.CreatePost(context.Background(), "channel-abc", attachment)
	require.NoError(t, err)
	assert.Equal(t, "post-123", postID)
	assert.Equal(t, "test-token-456", capturedToken)
	assert.Equal(t, "channel-abc", capturedRequest.ChannelID)
	assert.Equal(t, "", capturedRequest.Message)
	assert.NotNil(t, capturedRequest.Props)
	assert.Contains(t, capturedRequest.Props, "attachments")
}

func TestCreatePostServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "database error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	attachment := post.Attachment{Title: "Test"}
	postID, err := client.CreatePost(context.Background(), "channel-123", attachment)
	require.Error(t, err)
	assert.Empty(t, postID)
	assert.Contains(t, err.Error(), "status 500")
}

func TestCreatePostNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-token", logger)

	attachment := post.Attachment{Title: "Test"}
	postID, err := client.CreatePost(context.Background(), "channel-123", attachment)
	require.Error(t, err)
	assert.Empty(t, postID)
}

func TestUpdatePostSuccess(t *testing.T) {
	var capturedRequest updatePostRequest
	var capturedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/posts/post-456", r.URL.Path)
		capturedMethod = r.Method
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		authHeader := r.Header.Get("Authorization")
		require.Contains(t, authHeader, "Bearer test-token")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &capturedRequest)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "post-456"})
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	attachment := post.Attachment{
		Color: "#00FF00",
		Title: "Updated Alert",
	}

	err := client.UpdatePost(context.Background(), "post-456", attachment)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, capturedMethod)
	assert.Equal(t, "post-456", capturedRequest.ID)
	assert.NotNil(t, capturedRequest.Props)
}

func TestUpdatePostError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "post not found"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	attachment := post.Attachment{Title: "Test"}
	err := client.UpdatePost(context.Background(), "non-existent", attachment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

func TestGetUserSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/users/user-123", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		authHeader := r.Header.Get("Authorization")
		require.Contains(t, authHeader, "Bearer test-token")

		w.WriteHeader(http.StatusOK)
		response := userResponse{Username: "john.doe"}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	username, err := client.GetUser(context.Background(), "user-123")
	require.NoError(t, err)
	assert.Equal(t, "john.doe", username)
}

func TestGetUserError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "user not found"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	username, err := client.GetUser(context.Background(), "invalid-user")
	require.Error(t, err)
	assert.Empty(t, username)
	assert.Contains(t, err.Error(), "status 404")
}

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("https://mattermost.example.com", "token-123", logger)

	require.NotNil(t, client)
	assert.Equal(t, "https://mattermost.example.com", client.baseURL)
	assert.Equal(t, "token-123", client.token)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.logger)
}
