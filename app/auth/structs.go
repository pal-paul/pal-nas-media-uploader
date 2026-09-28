package auth

import "time"

type User struct {
	ID           string
	Username     string
	PasswordHash string
	TOTPSecret   string
}

type Challenge struct {
	UserID        string
	Username      string
	TOTPSecret    string
	PendingSecret string
	ExpiresAt     time.Time
}

type LoginChallenge struct {
	ChallengeToken  string `json:"challengeToken"`
	ProvisioningURI string `json:"provisioningUri,omitempty"`
	Secret          string `json:"secret,omitempty"`
}
