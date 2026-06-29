package domain

import "time"

type Session struct {
	ID          int       `json:"id"`
	HarnessName string    `json:"harness_name"`
	Role        string    `json:"role"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Provider    string    `json:"provider,omitempty"`
}
