package dto

type KeepAlertInput struct {
	ID          string `json:"id"          binding:"max=256"`
	Name        string `json:"name"        binding:"required,max=512"`
	Status      string `json:"status"      binding:"required,max=64"`
	Severity    string `json:"severity"    binding:"required,max=64"`
	Source      string `json:"source"      binding:"max=512"`
	Fingerprint string `json:"fingerprint" binding:"required,max=512"`
	Description string `json:"description" binding:"max=4096"`
	Labels      string `json:"labels"      binding:"max=65536"`
}
