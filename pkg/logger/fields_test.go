package logger

import (
	"log/slog"
	"testing"
)

func TestHTTPFields(t *testing.T) {
	attr := HTTPFields("req-123", "GET", "/api/test", "192.168.1.1", 200, 150, 100, 500)

	if attr.Key != "http" {
		t.Errorf("expected key 'http', got %s", attr.Key)
	}

	if attr.Value.Kind() != slog.KindGroup {
		t.Errorf("expected group kind, got %v", attr.Value.Kind())
	}

	groupAttrs := attr.Value.Group()
	expected := map[string]any{
		"request_id":    "req-123",
		"method":        "GET",
		"path":          "/api/test",
		"remote_ip":     "192.168.1.1",
		"status_code":   int64(200),
		"duration_ms":   int64(150),
		"request_size":  int64(100),
		"response_size": int64(500),
	}

	if len(groupAttrs) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(groupAttrs))
	}

	for _, a := range groupAttrs {
		expectedVal, ok := expected[a.Key]
		if !ok {
			t.Errorf("unexpected attribute key: %s", a.Key)
			continue
		}

		var actualVal any
		switch a.Value.Kind() {
		case slog.KindString:
			actualVal = a.Value.String()
		case slog.KindInt64:
			actualVal = a.Value.Int64()
		}

		if actualVal != expectedVal {
			t.Errorf("attribute %s: expected %v, got %v", a.Key, expectedVal, actualVal)
		}
	}
}

func TestExternalFields(t *testing.T) {
	attr := ExternalFields("keep-api", "https://api.example.com/v1", "POST", 201, 250)

	if attr.Key != "external" {
		t.Errorf("expected key 'external', got %s", attr.Key)
	}

	if attr.Value.Kind() != slog.KindGroup {
		t.Errorf("expected group kind, got %v", attr.Value.Kind())
	}

	groupAttrs := attr.Value.Group()
	expected := map[string]any{
		"service":     "keep-api",
		"url":         "https://api.example.com/v1",
		"method":      "POST",
		"status_code": int64(201),
		"duration_ms": int64(250),
	}

	if len(groupAttrs) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(groupAttrs))
	}

	for _, a := range groupAttrs {
		expectedVal, ok := expected[a.Key]
		if !ok {
			t.Errorf("unexpected attribute key: %s", a.Key)
			continue
		}

		var actualVal any
		switch a.Value.Kind() {
		case slog.KindString:
			actualVal = a.Value.String()
		case slog.KindInt64:
			actualVal = a.Value.Int64()
		}

		if actualVal != expectedVal {
			t.Errorf("attribute %s: expected %v, got %v", a.Key, expectedVal, actualVal)
		}
	}
}

func TestExternalFieldsWithError(t *testing.T) {
	attr := ExternalFieldsWithError("keep-api", "https://api.example.com/v1", "POST", 500, 100, "connection refused")

	if attr.Key != "external" {
		t.Errorf("expected key 'external', got %s", attr.Key)
	}

	if attr.Value.Kind() != slog.KindGroup {
		t.Errorf("expected group kind, got %v", attr.Value.Kind())
	}

	groupAttrs := attr.Value.Group()
	expected := map[string]any{
		"service":     "keep-api",
		"url":         "https://api.example.com/v1",
		"method":      "POST",
		"status_code": int64(500),
		"duration_ms": int64(100),
		"error":       "connection refused",
	}

	if len(groupAttrs) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(groupAttrs))
	}

	for _, a := range groupAttrs {
		expectedVal, ok := expected[a.Key]
		if !ok {
			t.Errorf("unexpected attribute key: %s", a.Key)
			continue
		}

		var actualVal any
		switch a.Value.Kind() {
		case slog.KindString:
			actualVal = a.Value.String()
		case slog.KindInt64:
			actualVal = a.Value.Int64()
		}

		if actualVal != expectedVal {
			t.Errorf("attribute %s: expected %v, got %v", a.Key, expectedVal, actualVal)
		}
	}
}

func TestRedisFields(t *testing.T) {
	attr := RedisFields("GET", "user:123", 5)

	if attr.Key != "redis" {
		t.Errorf("expected key 'redis', got %s", attr.Key)
	}

	if attr.Value.Kind() != slog.KindGroup {
		t.Errorf("expected group kind, got %v", attr.Value.Kind())
	}

	groupAttrs := attr.Value.Group()
	expected := map[string]any{
		"operation":   "GET",
		"key":         "user:123",
		"duration_ms": int64(5),
	}

	if len(groupAttrs) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(groupAttrs))
	}

	for _, a := range groupAttrs {
		expectedVal, ok := expected[a.Key]
		if !ok {
			t.Errorf("unexpected attribute key: %s", a.Key)
			continue
		}

		var actualVal any
		switch a.Value.Kind() {
		case slog.KindString:
			actualVal = a.Value.String()
		case slog.KindInt64:
			actualVal = a.Value.Int64()
		}

		if actualVal != expectedVal {
			t.Errorf("attribute %s: expected %v, got %v", a.Key, expectedVal, actualVal)
		}
	}
}

func TestRedisFieldsWithError(t *testing.T) {
	attr := RedisFieldsWithError("SET", "session:abc", 10, "redis: connection pool exhausted")

	if attr.Key != "redis" {
		t.Errorf("expected key 'redis', got %s", attr.Key)
	}

	if attr.Value.Kind() != slog.KindGroup {
		t.Errorf("expected group kind, got %v", attr.Value.Kind())
	}

	groupAttrs := attr.Value.Group()
	expected := map[string]any{
		"operation":   "SET",
		"key":         "session:abc",
		"duration_ms": int64(10),
		"error":       "redis: connection pool exhausted",
	}

	if len(groupAttrs) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(groupAttrs))
	}

	for _, a := range groupAttrs {
		expectedVal, ok := expected[a.Key]
		if !ok {
			t.Errorf("unexpected attribute key: %s", a.Key)
			continue
		}

		var actualVal any
		switch a.Value.Kind() {
		case slog.KindString:
			actualVal = a.Value.String()
		case slog.KindInt64:
			actualVal = a.Value.Int64()
		}

		if actualVal != expectedVal {
			t.Errorf("attribute %s: expected %v, got %v", a.Key, expectedVal, actualVal)
		}
	}
}

func TestApplicationFields(t *testing.T) {
	t.Run("with event only", func(t *testing.T) {
		attr := ApplicationFields("startup")

		if attr.Key != "application" {
			t.Errorf("expected key 'application', got %s", attr.Key)
		}

		if attr.Value.Kind() != slog.KindGroup {
			t.Errorf("expected group kind, got %v", attr.Value.Kind())
		}

		groupAttrs := attr.Value.Group()
		if len(groupAttrs) != 1 {
			t.Errorf("expected 1 attribute, got %d", len(groupAttrs))
		}

		if groupAttrs[0].Key != "event" {
			t.Errorf("expected key 'event', got %s", groupAttrs[0].Key)
		}

		if groupAttrs[0].Value.String() != "startup" {
			t.Errorf("expected value 'startup', got %s", groupAttrs[0].Value.String())
		}
	})

	t.Run("with additional attributes", func(t *testing.T) {
		attr := ApplicationFields("user_action",
			slog.String("user_id", "user-456"),
			slog.String("action", "login"),
			slog.Int("attempt", 3),
		)

		if attr.Key != "application" {
			t.Errorf("expected key 'application', got %s", attr.Key)
		}

		groupAttrs := attr.Value.Group()
		if len(groupAttrs) != 4 {
			t.Errorf("expected 4 attributes, got %d", len(groupAttrs))
		}

		expected := map[string]any{
			"event":   "user_action",
			"user_id": "user-456",
			"action":  "login",
			"attempt": int64(3),
		}

		for _, a := range groupAttrs {
			expectedVal, ok := expected[a.Key]
			if !ok {
				t.Errorf("unexpected attribute key: %s", a.Key)
				continue
			}

			var actualVal any
			switch a.Value.Kind() {
			case slog.KindString:
				actualVal = a.Value.String()
			case slog.KindInt64:
				actualVal = a.Value.Int64()
			}

			if actualVal != expectedVal {
				t.Errorf("attribute %s: expected %v, got %v", a.Key, expectedVal, actualVal)
			}
		}
	})
}
