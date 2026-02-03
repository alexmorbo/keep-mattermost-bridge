package dto

import (
	"encoding/json"
	"strings"
)

type KeepAlertInput struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"        binding:"required"`
	Status      string      `json:"status"      binding:"required"`
	Severity    string      `json:"severity"    binding:"required"`
	Source      FlexStrings `json:"source"`
	Fingerprint string      `json:"fingerprint" binding:"required"`
	Description string      `json:"description"`
	Labels      FlexLabels  `json:"labels"`
}

// FlexStrings handles both []string and Python list repr string like "['a', 'b']"
type FlexStrings []string

func (f *FlexStrings) UnmarshalJSON(data []byte) error {
	// Try native JSON array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = arr
		return nil
	}

	// Try string (Python repr format)
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*f = parsePythonList(s)
	return nil
}

// FlexLabels handles both map[string]string and Python dict repr string like "{'a': 'b'}"
type FlexLabels map[string]string

func (f *FlexLabels) UnmarshalJSON(data []byte) error {
	// Try native JSON object first
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		*f = m
		return nil
	}

	// Try string (Python repr format)
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*f = parsePythonDict(s)
	return nil
}

func parsePythonList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" || s == "None" {
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, "'\"")
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parsePythonDict(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" || s == "None" {
		return nil
	}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	result := make(map[string]string)
	for _, pair := range splitPythonPairs(s) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		key = strings.Trim(key, "'\"")
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "'\"")
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func splitPythonPairs(s string) []string {
	var pairs []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range s {
		switch {
		case (r == '\'' || r == '"') && !inQuote:
			inQuote = true
			quoteChar = r
			current.WriteRune(r)
		case r == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
			current.WriteRune(r)
		case r == ',' && !inQuote:
			pairs = append(pairs, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		pairs = append(pairs, current.String())
	}
	return pairs
}
