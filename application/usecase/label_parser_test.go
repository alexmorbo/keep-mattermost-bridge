package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePythonDictRepr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "valid Python dict with single quotes",
			input: "{'key1': 'value1', 'key2': 'value2'}",
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:  "valid Python dict with None value",
			input: "{'key': None}",
			expected: map[string]string{
				"key": "",
			},
		},
		{
			name:  "valid Python dict with True value",
			input: "{'key': True}",
			expected: map[string]string{
				"key": "true",
			},
		},
		{
			name:  "valid Python dict with False value",
			input: "{'key': False}",
			expected: map[string]string{
				"key": "false",
			},
		},
		{
			name:  "Python dict with nested single quotes",
			input: `{'key': "it's a value"}`,
			expected: map[string]string{
				"key": "it's a value",
			},
		},
		{
			name:  "Python dict with nested double quotes",
			input: `{"key": "value with \"quotes\""}`,
			expected: map[string]string{
				"key": `value with "quotes"`,
			},
		},
		{
			name:     "empty dict",
			input:    "{}",
			expected: map[string]string{},
		},
		{
			name:     "valid JSON",
			input:    `{"key": "value"}`,
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "Python dict with spaces",
			input:    "{ 'key' : 'value' }",
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "single entry",
			input:    "{'key': 'value'}",
			expected: map[string]string{"key": "value"},
		},
		{
			name:  "Python dict with multiple data types",
			input: "{'str': 'text', 'bool': True, 'none': None, 'bool2': False}",
			expected: map[string]string{
				"str":   "text",
				"bool":  "true",
				"none":  "",
				"bool2": "false",
			},
		},
		{
			name:  "Python dict with spaces in values",
			input: "{'key': 'value with spaces'}",
			expected: map[string]string{
				"key": "value with spaces",
			},
		},
		{
			name:  "Python dict with double quotes for keys",
			input: `{"key1": "value1", "key2": "value2"}`,
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:     "invalid string without braces",
			input:    "invalid",
			expected: map[string]string{},
		},
		{
			name:  "Python dict with mixed quotes",
			input: `{'key1': "value1", "key2": 'value2'}`,
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:  "escaped backslash before quote",
			input: `{'key': 'value with \\'}`,
			expected: map[string]string{
				"key": `value with \\`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePythonDictRepr(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
