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

func TestToWireAttachment_EmptyActions(t *testing.T) {
	attachment := post.Attachment{
		Color:      "#FF0000",
		Title:      "Test Alert",
		TitleLink:  "https://example.com",
		Text:       "Test message",
		Fields:     nil,
		Actions:    []post.Button{},
		Footer:     "Test Footer",
		FooterIcon: "https://example.com/icon.png",
	}

	wire := toWireAttachment(attachment)

	assert.Equal(t, "#FF0000", wire.Color)
	assert.Equal(t, "Test Alert", wire.Title)
	assert.Equal(t, "https://example.com", wire.TitleLink)
	assert.Equal(t, "Test message", wire.Text)
	assert.Empty(t, wire.Fields)
	assert.Empty(t, wire.Actions)
	assert.Equal(t, "Test Footer", wire.Footer)
	assert.Equal(t, "https://example.com/icon.png", wire.FooterIcon)
}

func TestToWireAttachment_MultipleFields(t *testing.T) {
	attachment := post.Attachment{
		Color: "#00FF00",
		Title: "Alert with Fields",
		Fields: []post.AttachmentField{
			{Title: "Field1", Value: "Value1", Short: true},
			{Title: "Field2", Value: "Value2", Short: false},
			{Title: "Field3", Value: "Value3", Short: true},
		},
		Actions: nil,
	}

	wire := toWireAttachment(attachment)

	require.Len(t, wire.Fields, 3)
	assert.Equal(t, "Field1", wire.Fields[0].Title)
	assert.Equal(t, "Value1", wire.Fields[0].Value)
	assert.True(t, wire.Fields[0].Short)
	assert.Equal(t, "Field2", wire.Fields[1].Title)
	assert.Equal(t, "Value2", wire.Fields[1].Value)
	assert.False(t, wire.Fields[1].Short)
	assert.Equal(t, "Field3", wire.Fields[2].Title)
	assert.Equal(t, "Value3", wire.Fields[2].Value)
	assert.True(t, wire.Fields[2].Short)
}

func TestToWireAttachment_AllFieldsPopulated(t *testing.T) {
	attachment := post.Attachment{
		Color:     "#0000FF",
		Title:     "Full Attachment",
		TitleLink: "https://example.com/alert",
		Text:      "Detailed description",
		Fields: []post.AttachmentField{
			{Title: "Severity", Value: "Critical", Short: true},
			{Title: "Source", Value: "Monitoring", Short: true},
		},
		Actions: []post.Button{
			{
				ID:   "btn-ack",
				Name: "Acknowledge",
				Integration: post.ButtonIntegration{
					URL:     "https://api.example.com/ack",
					Context: map[string]string{"alert_id": "123", "action": "ack"},
				},
			},
			{
				ID:   "btn-resolve",
				Name: "Resolve",
				Integration: post.ButtonIntegration{
					URL:     "https://api.example.com/resolve",
					Context: map[string]string{"alert_id": "123", "action": "resolve"},
				},
			},
		},
		Footer:     "Generated by Keep",
		FooterIcon: "https://example.com/keep-icon.png",
	}

	wire := toWireAttachment(attachment)

	assert.Equal(t, "#0000FF", wire.Color)
	assert.Equal(t, "Full Attachment", wire.Title)
	assert.Equal(t, "https://example.com/alert", wire.TitleLink)
	assert.Equal(t, "Detailed description", wire.Text)
	assert.Equal(t, "Generated by Keep", wire.Footer)
	assert.Equal(t, "https://example.com/keep-icon.png", wire.FooterIcon)

	require.Len(t, wire.Fields, 2)
	assert.Equal(t, "Severity", wire.Fields[0].Title)
	assert.Equal(t, "Critical", wire.Fields[0].Value)
	assert.True(t, wire.Fields[0].Short)

	require.Len(t, wire.Actions, 2)
	assert.Equal(t, "button", wire.Actions[0].Type)
	assert.Equal(t, "btn-ack", wire.Actions[0].ID)
	assert.Equal(t, "Acknowledge", wire.Actions[0].Name)
	assert.Equal(t, "https://api.example.com/ack", wire.Actions[0].Integration.URL)
	assert.Equal(t, "123", wire.Actions[0].Integration.Context["alert_id"])
	assert.Equal(t, "ack", wire.Actions[0].Integration.Context["action"])

	assert.Equal(t, "button", wire.Actions[1].Type)
	assert.Equal(t, "btn-resolve", wire.Actions[1].ID)
	assert.Equal(t, "Resolve", wire.Actions[1].Name)
}

func TestUpdatePostNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-token", logger)

	attachment := post.Attachment{Title: "Test"}
	err := client.UpdatePost(context.Background(), "post-123", attachment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mattermost update post")
}

func TestUpdatePostNon2xxStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"BadRequest", http.StatusBadRequest, `{"error": "invalid request"}`},
		{"Unauthorized", http.StatusUnauthorized, `{"error": "unauthorized"}`},
		{"Forbidden", http.StatusForbidden, `{"error": "forbidden"}`},
		{"InternalServerError", http.StatusInternalServerError, `{"error": "server error"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			client := NewClient(server.URL, "test-token", logger)

			attachment := post.Attachment{Title: "Test"}
			err := client.UpdatePost(context.Background(), "post-123", attachment)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "status")
		})
	}
}

func TestUpdatePostRequestCreationError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:8080", "test-token", logger)

	attachment := post.Attachment{Title: "Test"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.UpdatePost(ctx, "post-123", attachment)
	require.Error(t, err)
}

func TestGetUserNon200StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"BadRequest", http.StatusBadRequest, `{"error": "invalid request"}`},
		{"Unauthorized", http.StatusUnauthorized, `{"error": "unauthorized"}`},
		{"InternalServerError", http.StatusInternalServerError, `{"error": "server error"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			client := NewClient(server.URL, "test-token", logger)

			username, err := client.GetUser(context.Background(), "user-123")
			require.Error(t, err)
			assert.Empty(t, username)
			assert.Contains(t, err.Error(), "status")
		})
	}
}

func TestGetUserJSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	username, err := client.GetUser(context.Background(), "user-123")
	require.Error(t, err)
	assert.Empty(t, username)
	assert.Contains(t, err.Error(), "decode user response")
}

func TestGetUserEmptyUsername(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := userResponse{Username: ""}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	username, err := client.GetUser(context.Background(), "user-123")
	require.NoError(t, err)
	assert.Empty(t, username)
}

func TestGetUserNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-token", logger)

	username, err := client.GetUser(context.Background(), "user-123")
	require.Error(t, err)
	assert.Empty(t, username)
	assert.Contains(t, err.Error(), "mattermost get user")
}

func TestCreatePostJSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`invalid json response`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-token", logger)

	attachment := post.Attachment{Title: "Test"}
	postID, err := client.CreatePost(context.Background(), "channel-123", attachment)
	require.Error(t, err)
	assert.Empty(t, postID)
	assert.Contains(t, err.Error(), "decode create post response")
}
