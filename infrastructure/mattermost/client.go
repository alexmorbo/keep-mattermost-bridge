package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

var (
	mmCreatePostOK  = metrics.NewCounter(`mattermost_api_calls_total{operation="create_post",status="ok"}`)
	mmCreatePostErr = metrics.NewCounter(`mattermost_api_calls_total{operation="create_post",status="error"}`)
	mmCreatePostDur = metrics.NewHistogram(`mattermost_api_duration_seconds{operation="create_post"}`)

	mmUpdatePostOK  = metrics.NewCounter(`mattermost_api_calls_total{operation="update_post",status="ok"}`)
	mmUpdatePostErr = metrics.NewCounter(`mattermost_api_calls_total{operation="update_post",status="error"}`)
	mmUpdatePostDur = metrics.NewHistogram(`mattermost_api_duration_seconds{operation="update_post"}`)

	mmReplyToThreadOK  = metrics.NewCounter(`mattermost_api_calls_total{operation="reply_to_thread",status="ok"}`)
	mmReplyToThreadErr = metrics.NewCounter(`mattermost_api_calls_total{operation="reply_to_thread",status="error"}`)
	mmReplyToThreadDur = metrics.NewHistogram(`mattermost_api_duration_seconds{operation="reply_to_thread"}`)
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(baseURL, token string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: logger,
	}
}

type createPostRequest struct {
	ChannelID string         `json:"channel_id"`
	Message   string         `json:"message"`
	Props     map[string]any `json:"props,omitempty"`
}

type createPostResponse struct {
	ID string `json:"id"`
}

type updatePostRequest struct {
	ID      string         `json:"id"`
	Message string         `json:"message"`
	Props   map[string]any `json:"props,omitempty"`
}

type replyPostRequest struct {
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
	Message   string `json:"message"`
}

type userResponse struct {
	Username string `json:"username"`
}

type wireAttachment struct {
	Color      string       `json:"color,omitempty"`
	Title      string       `json:"title,omitempty"`
	TitleLink  string       `json:"title_link,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []wireField  `json:"fields,omitempty"`
	Actions    []wireButton `json:"actions,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
}

type wireField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type wireButton struct {
	Type        string                `json:"type"`
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Style       string                `json:"style,omitempty"`
	Integration wireButtonIntegration `json:"integration"`
}

type wireButtonIntegration struct {
	URL     string            `json:"url"`
	Context map[string]string `json:"context"`
}

func toWireAttachment(a post.Attachment) wireAttachment {
	fields := make([]wireField, len(a.Fields))
	for i, f := range a.Fields {
		fields[i] = wireField{Title: f.Title, Value: f.Value, Short: f.Short}
	}

	buttons := make([]wireButton, len(a.Actions))
	for i, b := range a.Actions {
		buttons[i] = wireButton{
			Type:  "button",
			ID:    b.ID,
			Name:  b.Name,
			Style: b.Style,
			Integration: wireButtonIntegration{
				URL:     b.Integration.URL,
				Context: b.Integration.Context,
			},
		}
	}

	return wireAttachment{
		Color:      a.Color,
		Title:      a.Title,
		TitleLink:  a.TitleLink,
		Text:       a.Text,
		Fields:     fields,
		Actions:    buttons,
		Footer:     a.Footer,
		FooterIcon: a.FooterIcon,
	}
}

func (c *Client) CreatePost(ctx context.Context, channelID string, attachment post.Attachment) (string, error) {
	start := time.Now()
	reqURL := c.baseURL + "/api/v4/posts"

	body := createPostRequest{
		ChannelID: channelID,
		Message:   "",
		Props: map[string]any{
			"attachments": []wireAttachment{toWireAttachment(attachment)},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal post body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Mattermost CreatePost failed",
			logger.ExternalFieldsWithError("mattermost", reqURL, "POST", 0, duration, err.Error()),
		)
		mmCreatePostErr.Inc()
		return "", fmt.Errorf("mattermost create post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Mattermost CreatePost non-201",
			logger.ExternalFieldsWithError("mattermost", reqURL, "POST", resp.StatusCode, duration, string(respBody)),
		)
		mmCreatePostErr.Inc()
		return "", fmt.Errorf("mattermost create post: status %d, body: %s", resp.StatusCode, respBody)
	}

	var result createPostResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create post response: %w", err)
	}

	c.logger.Debug("Mattermost CreatePost completed",
		logger.ExternalFields("mattermost", reqURL, "POST", resp.StatusCode, duration),
	)
	mmCreatePostOK.Inc()
	mmCreatePostDur.Update(float64(duration) / 1000)

	return result.ID, nil
}

func (c *Client) UpdatePost(ctx context.Context, postID string, attachment post.Attachment) error {
	start := time.Now()
	reqURL := c.baseURL + "/api/v4/posts/" + url.PathEscape(postID)

	body := updatePostRequest{
		ID:      postID,
		Message: "",
		Props: map[string]any{
			"attachments": []wireAttachment{toWireAttachment(attachment)},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal update body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Mattermost UpdatePost failed",
			logger.ExternalFieldsWithError("mattermost", reqURL, "PUT", 0, duration, err.Error()),
		)
		mmUpdatePostErr.Inc()
		return fmt.Errorf("mattermost update post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Mattermost UpdatePost non-200",
			logger.ExternalFieldsWithError("mattermost", reqURL, "PUT", resp.StatusCode, duration, string(respBody)),
		)
		mmUpdatePostErr.Inc()
		return fmt.Errorf("mattermost update post: status %d, body: %s", resp.StatusCode, respBody)
	}

	c.logger.Debug("Mattermost UpdatePost completed",
		logger.ExternalFields("mattermost", reqURL, "PUT", resp.StatusCode, duration),
	)
	mmUpdatePostOK.Inc()
	mmUpdatePostDur.Update(float64(duration) / 1000)

	return nil
}

func (c *Client) GetUser(ctx context.Context, userID string) (string, error) {
	start := time.Now()
	reqURL := c.baseURL + "/api/v4/users/" + url.PathEscape(userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Mattermost GetUser failed",
			logger.ExternalFieldsWithError("mattermost", reqURL, "GET", 0, duration, err.Error()),
		)
		return "", fmt.Errorf("mattermost get user: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("mattermost get user: status %d, body: %s", resp.StatusCode, respBody)
	}

	var result userResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode user response: %w", err)
	}

	c.logger.Debug("Mattermost GetUser completed",
		logger.ExternalFields("mattermost", reqURL, "GET", resp.StatusCode, duration),
	)

	return result.Username, nil
}

func (c *Client) ReplyToThread(ctx context.Context, channelID, rootID, message string) error {
	start := time.Now()
	reqURL := c.baseURL + "/api/v4/posts"

	body := replyPostRequest{
		ChannelID: channelID,
		RootID:    rootID,
		Message:   message,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal reply body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Mattermost ReplyToThread failed",
			logger.ExternalFieldsWithError("mattermost", reqURL, "POST", 0, duration, err.Error()),
		)
		mmReplyToThreadErr.Inc()
		return fmt.Errorf("mattermost reply to thread: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Mattermost ReplyToThread non-201",
			logger.ExternalFieldsWithError("mattermost", reqURL, "POST", resp.StatusCode, duration, string(respBody)),
		)
		mmReplyToThreadErr.Inc()
		return fmt.Errorf("mattermost reply to thread: status %d, body: %s", resp.StatusCode, respBody)
	}

	c.logger.Debug("Mattermost ReplyToThread completed",
		logger.ExternalFields("mattermost", reqURL, "POST", resp.StatusCode, duration),
	)
	mmReplyToThreadOK.Inc()
	mmReplyToThreadDur.Update(float64(duration) / 1000)

	return nil
}
