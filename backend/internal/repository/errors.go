package repository

import "fmt"

// ErrNotFound represents a resource not found error in the repository layer.
type ErrNotFound struct {
	Resource string // The type of resource (e.g., "node", "edge")
	ID       string // The identifier that was not found
	UserID   string // The user context, if applicable
}

func (e ErrNotFound) Error() string {
	if e.UserID != "" {
		return fmt.Sprintf("%s with ID '%s' not found for user '%s'", e.Resource, e.ID, e.UserID)
	}
	return fmt.Sprintf("%s with ID '%s' not found", e.Resource, e.ID)
}

// IsNotFound checks if an error is a repository not found error.
func IsNotFound(err error) bool {
	_, ok := err.(ErrNotFound)
	return ok
}

// ErrConflict represents a conflict error in the repository layer.
type ErrConflict struct {
	Resource string // The type of resource (e.g., "node", "edge")
	ID       string // The identifier that caused the conflict
	Reason   string // The reason for the conflict
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("conflict with %s '%s': %s", e.Resource, e.ID, e.Reason)
}

// IsConflict checks if an error is a repository conflict error.
func IsConflict(err error) bool {
	_, ok := err.(ErrConflict)
	return ok
}

// ErrInvalidQuery represents an invalid query error in the repository layer.
type ErrInvalidQuery struct {
	Field  string // The field that caused the invalid query
	Reason string // The reason why the query is invalid
}

func (e ErrInvalidQuery) Error() string {
	return fmt.Sprintf("invalid query for field '%s': %s", e.Field, e.Reason)
}

// IsInvalidQuery checks if an error is a repository invalid query error.
func IsInvalidQuery(err error) bool {
	_, ok := err.(ErrInvalidQuery)
	return ok
}

// NewNotFound creates a new ErrNotFound.
func NewNotFound(resource, id string) ErrNotFound {
	return ErrNotFound{Resource: resource, ID: id}
}

// NewNotFoundWithUser creates a new ErrNotFound with user context.
func NewNotFoundWithUser(resource, id, userID string) ErrNotFound {
	return ErrNotFound{Resource: resource, ID: id, UserID: userID}
}

// NewConflict creates a new ErrConflict.
func NewConflict(resource, id, reason string) ErrConflict {
	return ErrConflict{Resource: resource, ID: id, Reason: reason}
}

// NewInvalidQuery creates a new ErrInvalidQuery.
func NewInvalidQuery(field, reason string) ErrInvalidQuery {
	return ErrInvalidQuery{Field: field, Reason: reason}
}