package domain

// RoleView is the admin role shape.
type RoleView struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsSystem    bool    `json:"is_system"`
}

// RoleList is the admin role collection.
type RoleList struct {
	Items []RoleView `json:"items"`
}

// CreateRoleRequest creates a realm-scoped role.
type CreateRoleRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}
