package schema

// ColumnType is the categorized data type of a column (e.g. INT, VARCHAR).
// Driver packages define the concrete constants that map onto these values.
type ColumnType string

// Column is a database-agnostic description of a single table column.
//
// Definition holds the full driver-native type as reported by the database
// (e.g. "varchar(255)", "int unsigned", "decimal(10,2)"). It is the value
// used when diffing types and when rendering DDL, since it preserves length,
// precision, sign and other modifiers exactly as the engine stores them.
type Column struct {
	Name       string     `json:"name" yaml:"name"`
	Type       ColumnType `json:"type" yaml:"type"`
	Definition string     `json:"definition" yaml:"definition"`
	Nullable   bool       `json:"nullable" yaml:"nullable"`
	Default    *string    `json:"default,omitempty" yaml:"default,omitempty"`
	Extra      string     `json:"extra,omitempty" yaml:"extra,omitempty"` // e.g. "auto_increment", "on update CURRENT_TIMESTAMP"
	Position   int        `json:"position,omitempty" yaml:"position,omitempty"`
	Charset    string     `json:"charset,omitempty" yaml:"charset,omitempty"`
	Collation  string     `json:"collation,omitempty" yaml:"collation,omitempty"`
}

// Equal reports whether two columns are identical for migration purposes.
// Position is intentionally ignored: column ordering does not, on its own,
// warrant an ALTER statement.
func (c *Column) Equal(other *Column) bool {
	if c == nil || other == nil {
		return c == other
	}
	if c.Name != other.Name ||
		c.Definition != other.Definition ||
		c.Nullable != other.Nullable ||
		c.Extra != other.Extra ||
		c.Charset != other.Charset ||
		c.Collation != other.Collation {
		return false
	}
	return defaultsEqual(c.Default, other.Default)
}

func defaultsEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

type InvalidColumnTypeError struct {
	ColumnType ColumnType
}

func (e InvalidColumnTypeError) Error() string {
	return "invalid column type: " + string(e.ColumnType)
}
