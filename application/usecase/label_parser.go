package usecase

import (
	"encoding/json"
	"strings"
)

func ParsePythonDictRepr(s string) map[string]string {
	if s == "" {
		return make(map[string]string)
	}

	s = strings.TrimSpace(s)

	// Try JSON first
	var jsonResult map[string]string
	if err := json.Unmarshal([]byte(s), &jsonResult); err == nil {
		return jsonResult
	}

	// Try JSON with interface values
	var jsonAny map[string]any
	if err := json.Unmarshal([]byte(s), &jsonAny); err == nil {
		result := make(map[string]string, len(jsonAny))
		for k, v := range jsonAny {
			switch val := v.(type) {
			case string:
				result[k] = val
			case nil:
				result[k] = ""
			default:
				b, _ := json.Marshal(val)
				result[k] = string(b)
			}
		}
		return result
	}

	// Parse Python dict repr
	return parsePythonDict(s)
}

func parsePythonDict(s string) map[string]string {
	result := make(map[string]string)

	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return result
	}

	// Remove outer braces
	s = s[1 : len(s)-1]
	s = strings.TrimSpace(s)

	if s == "" {
		return result
	}

	pairs := splitPairs(s)
	for _, pair := range pairs {
		key, value := splitKeyValue(pair)
		if key != "" {
			result[key] = value
		}
	}

	return result
}

func isEscaped(s string, pos int) bool {
	count := 0
	for i := pos - 1; i >= 0 && s[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func splitPairs(s string) []string {
	var pairs []string
	var current strings.Builder
	depth := 0
	inString := false
	var stringChar byte

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inString {
			current.WriteByte(ch)
			if ch == stringChar && !isEscaped(s, i) {
				inString = false
			}
			continue
		}

		switch ch {
		case '\'', '"':
			inString = true
			stringChar = ch
			current.WriteByte(ch)
		case '{', '[', '(':
			depth++
			current.WriteByte(ch)
		case '}', ']', ')':
			depth--
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				pairs = append(pairs, strings.TrimSpace(current.String()))
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		pairs = append(pairs, strings.TrimSpace(current.String()))
	}

	return pairs
}

func splitKeyValue(pair string) (string, string) {
	colonIdx := findColon(pair)
	if colonIdx < 0 {
		return "", ""
	}

	key := strings.TrimSpace(pair[:colonIdx])
	value := strings.TrimSpace(pair[colonIdx+1:])

	key = unquote(key)
	value = unquotePythonValue(value)

	return key, value
}

func findColon(s string) int {
	inString := false
	var stringChar byte

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inString {
			if ch == stringChar && !isEscaped(s, i) {
				inString = false
			}
			continue
		}

		switch ch {
		case '\'', '"':
			inString = true
			stringChar = ch
		case ':':
			return i
		}
	}

	return -1
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func unquotePythonValue(s string) string {
	if s == "None" || s == "none" {
		return ""
	}
	if s == "True" || s == "true" {
		return "true"
	}
	if s == "False" || s == "false" {
		return "false"
	}
	return unquote(s)
}
