package logger

import "log/slog"

func HTTPFields(requestID, method, path, remoteIP string, statusCode int, durationMs int64, requestSize, responseSize int) slog.Attr {
	return slog.Group("http",
		slog.String("request_id", requestID),
		slog.String("method", method),
		slog.String("path", path),
		slog.String("remote_ip", remoteIP),
		slog.Int("status_code", statusCode),
		slog.Int64("duration_ms", durationMs),
		slog.Int("request_size", requestSize),
		slog.Int("response_size", responseSize),
	)
}

func ExternalFields(service, url, method string, statusCode int, durationMs int64) slog.Attr {
	return slog.Group("external",
		slog.String("service", service),
		slog.String("url", url),
		slog.String("method", method),
		slog.Int("status_code", statusCode),
		slog.Int64("duration_ms", durationMs),
	)
}

func ExternalFieldsWithError(service, url, method string, statusCode int, durationMs int64, errMsg string) slog.Attr {
	return slog.Group("external",
		slog.String("service", service),
		slog.String("url", url),
		slog.String("method", method),
		slog.Int("status_code", statusCode),
		slog.Int64("duration_ms", durationMs),
		slog.String("error", errMsg),
	)
}

func RedisFields(operation, key string, durationMs int64) slog.Attr {
	return slog.Group("redis",
		slog.String("operation", operation),
		slog.String("key", key),
		slog.Int64("duration_ms", durationMs),
	)
}

func RedisFieldsWithError(operation, key string, durationMs int64, errMsg string) slog.Attr {
	return slog.Group("redis",
		slog.String("operation", operation),
		slog.String("key", key),
		slog.Int64("duration_ms", durationMs),
		slog.String("error", errMsg),
	)
}

func ApplicationFields(event string, attrs ...slog.Attr) slog.Attr {
	args := make([]any, 0, len(attrs)+1)
	args = append(args, slog.String("event", event))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	return slog.Group("application", args...)
}
