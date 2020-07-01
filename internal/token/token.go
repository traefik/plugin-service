package token

import "time"

// Token the token.
type Token struct {
	ID        string    `json:"id,omitempty"`
	Value     string    `json:"value,omitempty"`
	Name      string    `json:"name,omitempty"`
	UserID    string    `json:"userID,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}
