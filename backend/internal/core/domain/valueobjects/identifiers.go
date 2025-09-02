// Package valueobjects contains domain value objects that encapsulate
// business concepts and rules without having their own identity.
// Value objects are immutable and compared by value equality.
package valueobjects

import (
	"fmt"
	"regexp"
	"strings"
	
	"github.com/google/uuid"
)

// NodeID represents a unique identifier for a node
type NodeID struct {
	value string
}

// NewNodeID creates a new random NodeID
func NewNodeID(value string) NodeID {
	if value == "" {
		value = uuid.New().String()
	}
	return NodeID{value: value}
}

// String returns the string representation
func (id NodeID) String() string {
	return id.value
}

// Equals checks equality with another NodeID
func (id NodeID) Equals(other NodeID) bool {
	return id.value == other.value
}

// Validate checks if the NodeID is valid
func (id NodeID) Validate() error {
	if id.value == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	if len(id.value) > 128 {
		return fmt.Errorf("node ID too long")
	}
	return nil
}

// UserID represents a unique identifier for a user
type UserID struct {
	value string
}

// NewUserID creates a new UserID
func NewUserID(value string) UserID {
	return UserID{value: value}
}

// String returns the string representation
func (id UserID) String() string {
	return id.value
}

// Equals checks equality with another UserID
func (id UserID) Equals(other UserID) bool {
	return id.value == other.value
}

// Validate checks if the UserID is valid
func (id UserID) Validate() error {
	if id.value == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if len(id.value) > 128 {
		return fmt.Errorf("user ID too long")
	}
	return nil
}

// CategoryID represents a unique identifier for a category
type CategoryID struct {
	value string
}

// NewCategoryID creates a new CategoryID
func NewCategoryID(value string) CategoryID {
	if value == "" {
		value = uuid.New().String()
	}
	return CategoryID{value: value}
}

// String returns the string representation
func (id CategoryID) String() string {
	return id.value
}

// Equals checks equality with another CategoryID
func (id CategoryID) Equals(other CategoryID) bool {
	return id.value == other.value
}

// Validate checks if the CategoryID is valid
func (id CategoryID) Validate() error {
	if id.value == "" {
		return fmt.Errorf("category ID cannot be empty")
	}
	if len(id.value) > 128 {
		return fmt.Errorf("category ID too long")
	}
	return nil
}

// EdgeID represents a unique identifier for an edge
type EdgeID struct {
	value string
}

// NewEdgeID creates a new EdgeID
func NewEdgeID(value string) EdgeID {
	if value == "" {
		value = uuid.New().String()
	}
	return EdgeID{value: value}
}

// String returns the string representation
func (id EdgeID) String() string {
	return id.value
}

// Equals checks equality with another EdgeID
func (id EdgeID) Equals(other EdgeID) bool {
	return id.value == other.value
}

// Validate checks if the EdgeID is valid
func (id EdgeID) Validate() error {
	if id.value == "" {
		return fmt.Errorf("edge ID cannot be empty")
	}
	if len(id.value) > 128 {
		return fmt.Errorf("edge ID too long")
	}
	return nil
}

// CorrelationID represents a correlation ID for distributed tracing
type CorrelationID struct {
	value string
}

// NewCorrelationID creates a new CorrelationID
func NewCorrelationID() CorrelationID {
	return CorrelationID{value: uuid.New().String()}
}

// NewCorrelationIDFromString creates a CorrelationID from a string
func NewCorrelationIDFromString(value string) CorrelationID {
	if value == "" {
		return NewCorrelationID()
	}
	return CorrelationID{value: value}
}

// String returns the string representation
func (id CorrelationID) String() string {
	return id.value
}

// Equals checks equality with another CorrelationID
func (id CorrelationID) Equals(other CorrelationID) bool {
	return id.value == other.value
}

// Email represents an email address with validation
type Email struct {
	value string
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// NewEmail creates a new Email value object
func NewEmail(value string) (Email, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	email := Email{value: value}
	if err := email.Validate(); err != nil {
		return Email{}, err
	}
	return email, nil
}

// String returns the string representation
func (e Email) String() string {
	return e.value
}

// Equals checks equality with another Email
func (e Email) Equals(other Email) bool {
	return e.value == other.value
}

// Validate checks if the email is valid
func (e Email) Validate() error {
	if e.value == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if !emailRegex.MatchString(e.value) {
		return fmt.Errorf("invalid email format")
	}
	if len(e.value) > 255 {
		return fmt.Errorf("email too long")
	}
	return nil
}

// Domain returns the domain part of the email
func (e Email) Domain() string {
	parts := strings.Split(e.value, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// Username returns the username part of the email
func (e Email) Username() string {
	parts := strings.Split(e.value, "@")
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}