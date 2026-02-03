package logger

import (
	"context"
	"testing"
)

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "req-abc-123"

	newCtx := WithRequestID(ctx, requestID)

	if newCtx == nil {
		t.Fatal("expected context to not be nil")
	}

	if newCtx == ctx {
		t.Error("expected new context to be different from original")
	}

	storedID := newCtx.Value(requestIDKey)
	if storedID != requestID {
		t.Errorf("expected stored request ID to be %s, got %v", requestID, storedID)
	}
}

func TestGetRequestID(t *testing.T) {
	t.Run("returns request ID when present", func(t *testing.T) {
		ctx := context.Background()
		requestID := "req-xyz-789"
		ctx = WithRequestID(ctx, requestID)

		result := GetRequestID(ctx)

		if result != requestID {
			t.Errorf("expected %s, got %s", requestID, result)
		}
	})

	t.Run("returns empty string when not present", func(t *testing.T) {
		ctx := context.Background()

		result := GetRequestID(ctx)

		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})

	t.Run("returns empty string when value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestIDKey, 12345)

		result := GetRequestID(ctx)

		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})

}

func TestWithRequestIDAndGetRequestIDIntegration(t *testing.T) {
	ctx := context.Background()

	id1 := GetRequestID(ctx)
	if id1 != "" {
		t.Errorf("expected empty string initially, got %s", id1)
	}

	ctx = WithRequestID(ctx, "first-request")
	id2 := GetRequestID(ctx)
	if id2 != "first-request" {
		t.Errorf("expected 'first-request', got %s", id2)
	}

	ctx = WithRequestID(ctx, "second-request")
	id3 := GetRequestID(ctx)
	if id3 != "second-request" {
		t.Errorf("expected 'second-request', got %s", id3)
	}
}

type testStringKey string

func TestContextKeyType(t *testing.T) {
	ctx := context.Background()

	ctx = context.WithValue(ctx, testStringKey("request_id"), "string-key-value")

	ctx = WithRequestID(ctx, "typed-key-value")

	result := GetRequestID(ctx)
	if result != "typed-key-value" {
		t.Errorf("expected 'typed-key-value', got %s", result)
	}

	stringKeyValue := ctx.Value(testStringKey("request_id"))
	if stringKeyValue != "string-key-value" {
		t.Errorf("expected string key to still have its value, got %v", stringKeyValue)
	}
}
