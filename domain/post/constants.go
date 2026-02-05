package post

const (
	ActionAcknowledge   = "acknowledge"
	ActionResolve       = "resolve"
	ActionUnacknowledge = "unacknowledge"
)

const (
	ButtonStyleDefault = "default"
	ButtonStyleSuccess = "success"
	ButtonStyleDanger  = "danger"
)

const (
	ContextKeyAction         = "action"
	ContextKeyFingerprint    = "fingerprint"
	ContextKeyAlertName      = "alert_name"
	ContextKeySeverity       = "severity"
	ContextKeyAttachmentJSON = "attachment_json"
)

const (
	SeverityPositionFirst        = "first"
	SeverityPositionAfterDisplay = "after_display"
	SeverityPositionLast         = "last"
)
