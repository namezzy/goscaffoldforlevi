package org

import "github.com/google/uuid"

// NewID mints a UUIDv7 for a new department. Generation lives here, not in
// handlers or the repository, matching user.NewID (ADR-0006): an entity must
// have its identity before it reaches the database.
func NewID() string { return uuid.Must(uuid.NewV7()).String() }
