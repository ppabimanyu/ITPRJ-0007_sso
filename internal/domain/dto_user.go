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

// AdminUserView is the console shape: the public user plus live session count.
type AdminUserView struct {
	UserDTO
	SessionCount int `json:"session_count"`
}

// AdminUserList is the keyset-paginated admin user page.
type AdminUserList struct {
	Items     []AdminUserView `json:"items"`
	NextAfter *string         `json:"next_after"`
}

// AdminUserFilter drives the admin user query.
type AdminUserFilter struct {
	Query  string
	Status string
	After  string
	Limit  int
}

// AdminUserUpdate is the partial update accepted by PATCH /users/{id}.
type AdminUserUpdate struct {
	Name       *string
	GivenName  *string
	FamilyName *string
	Locale     *string
	Disabled   *bool
}

// CreateUserRequest creates an invite-only user.
type CreateUserRequest struct {
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	Username   *string `json:"username,omitempty"`
	SendInvite *bool   `json:"send_invite,omitempty"`
}

// UpdateUserRequest is the PATCH /users/{id} body.
type UpdateUserRequest struct {
	Name       *string `json:"name,omitempty"`
	GivenName  *string `json:"given_name,omitempty"`
	FamilyName *string `json:"family_name,omitempty"`
	Locale     *string `json:"locale,omitempty"`
	Disabled   *bool   `json:"disabled,omitempty"`
}

// AssignRolesRequest replaces a user's role set.
type AssignRolesRequest struct {
	RoleIDs []string `json:"role_ids"`
}

// RolesResponse is the role list returned after replacement.
type RolesResponse struct {
	Roles []RoleRef `json:"roles"`
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
