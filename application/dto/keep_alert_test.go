package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexStrings_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FlexStrings
		wantErr  bool
	}{
		{
			name:     "json string value",
			input:    `"prometheus"`,
			expected: FlexStrings{"prometheus"},
		},
		{
			name:     "json array",
			input:    `["prometheus", "alertmanager"]`,
			expected: FlexStrings{"prometheus", "alertmanager"},
		},
		{
			name:     "python-like list",
			input:    `"['prometheus', 'alertmanager']"`,
			expected: FlexStrings{"prometheus", "alertmanager"},
		},
		{
			name:     "python-like list with double quotes",
			input:    `"[\"prometheus\", \"alertmanager\"]"`,
			expected: FlexStrings{"prometheus", "alertmanager"},
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: nil,
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: FlexStrings{},
		},
		{
			name:     "python empty list string",
			input:    `"[]"`,
			expected: nil,
		},
		{
			name:     "python None",
			input:    `"None"`,
			expected: nil,
		},
		{
			name:     "single element array",
			input:    `["single"]`,
			expected: FlexStrings{"single"},
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result FlexStrings
			err := json.Unmarshal([]byte(tt.input), &result)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFlexLabels_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FlexLabels
		wantErr  bool
	}{
		{
			name:     "normal json object",
			input:    `{"key": "value"}`,
			expected: FlexLabels{"key": "value"},
		},
		{
			name:     "json object with multiple keys",
			input:    `{"key1": "value1", "key2": "value2"}`,
			expected: FlexLabels{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "python-like dict with single quotes",
			input:    `"{'key': 'value'}"`,
			expected: FlexLabels{"key": "value"},
		},
		{
			name:     "python-like dict with multiple keys",
			input:    `"{'key1': 'value1', 'key2': 'value2'}"`,
			expected: FlexLabels{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: FlexLabels{},
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: nil,
		},
		{
			name:     "python empty dict string",
			input:    `"{}"`,
			expected: nil,
		},
		{
			name:     "python None",
			input:    `"None"`,
			expected: nil,
		},
		{
			name:     "nested value with colon",
			input:    `"{'url': 'http://example.com:8080'}"`,
			expected: FlexLabels{"url": "http://example.com:8080"},
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result FlexLabels
			err := json.Unmarshal([]byte(tt.input), &result)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeepAlertInput_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected KeepAlertInput
		wantErr  bool
	}{
		{
			name: "full alert with native json types",
			input: `{
				"id": "alert-123",
				"name": "HighCPU",
				"status": "firing",
				"severity": "critical",
				"source": ["prometheus"],
				"fingerprint": "abc123",
				"description": "CPU is high",
				"labels": {"instance": "server1"},
				"firingStartTime": "2024-01-01T00:00:00Z"
			}`,
			expected: KeepAlertInput{
				ID:              "alert-123",
				Name:            "HighCPU",
				Status:          "firing",
				Severity:        "critical",
				Source:          FlexStrings{"prometheus"},
				Fingerprint:     "abc123",
				Description:     "CPU is high",
				Labels:          FlexLabels{"instance": "server1"},
				FiringStartTime: "2024-01-01T00:00:00Z",
			},
		},
		{
			name: "alert with python-like source and labels",
			input: `{
				"id": "alert-456",
				"name": "HighMemory",
				"status": "resolved",
				"severity": "warning",
				"source": "['prometheus', 'alertmanager']",
				"fingerprint": "def456",
				"description": "Memory is high",
				"labels": "{'instance': 'server2', 'job': 'node'}",
				"firingStartTime": "2024-01-02T00:00:00Z"
			}`,
			expected: KeepAlertInput{
				ID:              "alert-456",
				Name:            "HighMemory",
				Status:          "resolved",
				Severity:        "warning",
				Source:          FlexStrings{"prometheus", "alertmanager"},
				Fingerprint:     "def456",
				Description:     "Memory is high",
				Labels:          FlexLabels{"instance": "server2", "job": "node"},
				FiringStartTime: "2024-01-02T00:00:00Z",
			},
		},
		{
			name: "alert with empty optional fields",
			input: `{
				"name": "TestAlert",
				"status": "firing",
				"severity": "info",
				"fingerprint": "ghi789"
			}`,
			expected: KeepAlertInput{
				Name:        "TestAlert",
				Status:      "firing",
				Severity:    "info",
				Fingerprint: "ghi789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result KeepAlertInput
			err := json.Unmarshal([]byte(tt.input), &result)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
