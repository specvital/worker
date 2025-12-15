package analysis

import "github.com/google/uuid"

// UUID is a database-agnostic unique identifier using google/uuid.
type UUID = uuid.UUID

var NilUUID = uuid.Nil

func NewUUID() UUID {
	return uuid.New()
}

func ParseUUID(s string) (UUID, error) {
	return uuid.Parse(s)
}
