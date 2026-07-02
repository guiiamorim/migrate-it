// Package reflection reads a live database and produces a driver-agnostic
// *schema.Schema that can be diffed and migrated.
package reflection

import "github.com/guiiamorim/migrateit/internal/schema"

// Reflector introspects a connected database into a schema snapshot.
type Reflector interface {
	Reflect() (*schema.Schema, error)
}
