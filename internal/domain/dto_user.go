package domain

import "time"

// RoleRef is the compact role shape returned in a user body.
type RoleRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GroupRef is the compact group shape returned in a user body.
type GroupRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserDTO is the public user representation from the OpenAPI schema.
type UserDTO struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Username        *string    `json:"username"`
	Name            *string    `json:"name"`
	GivenName       *string    `json:"given_name"`
	FamilyName      *string    `json:"family_name"`
	Locale          *string    `json:"locale"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	DisabledAt      *time.Time `json:"disabled_at"`
	MFAEnrolled     bool       `json:"mfa_enrolled"`
	Roles           []RoleRef  `json:"roles"`
	Groups          []GroupRef `json:"groups"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
