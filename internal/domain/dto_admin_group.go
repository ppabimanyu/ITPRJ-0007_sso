package domain

// GroupView is the admin group shape with a live member count.
type GroupView struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	MemberCount int     `json:"member_count"`
}

// GroupList is the admin group collection.
type GroupList struct {
	Items []GroupView `json:"items"`
}

// CreateGroupRequest creates a flat group.
type CreateGroupRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// UpdateGroupRequest is the PATCH /groups/{id} body.
type UpdateGroupRequest struct {
	Name        *string          `json:"name,omitempty"`
	Description Optional[string] `json:"description,omitzero"`
}

// GroupUpdate is the repository-level group patch.
type GroupUpdate struct {
	Name        *string
	Description Optional[string]
}

// GroupMember is one user in a group.
type GroupMember struct {
	UserID string  `json:"user_id"`
	Email  string  `json:"email"`
	Name   *string `json:"name"`
}

// GroupMemberList is the group member collection.
type GroupMemberList struct {
	Items []GroupMember `json:"items"`
}

// AddGroupMembersRequest adds one or more users to a group.
type AddGroupMembersRequest struct {
	UserIDs []string `json:"user_ids"`
}
