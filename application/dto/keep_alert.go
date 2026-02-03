package dto

type KeepAlertInput struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"        binding:"required"`
	Status      string            `json:"status"      binding:"required"`
	Severity    string            `json:"severity"    binding:"required"`
	Source      []string          `json:"source"`
	Fingerprint string            `json:"fingerprint" binding:"required"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
}
