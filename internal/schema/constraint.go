package schema

import "strings"

// ConstraintType enumerates the kinds of constraints/indexes a table can carry.
// Foreign keys are modelled separately (see ForeignKey) because they alone
// create inter-table precedence that the migration graph must order.
type ConstraintType string

// Constraint describes a primary key, unique key, plain index or check.
type Constraint struct {
	Name       string         `json:"name" yaml:"name"`
	Type       ConstraintType `json:"type" yaml:"type"`
	Columns    []string       `json:"columns,omitempty" yaml:"columns,omitempty"`
	Expression string         `json:"expression,omitempty" yaml:"expression,omitempty"` // for CHECK constraints
}

// Equal reports whether two constraints are structurally identical.
func (c *Constraint) Equal(other *Constraint) bool {
	if c == nil || other == nil {
		return c == other
	}
	return c.Type == other.Type &&
		c.Expression == other.Expression &&
		slicesEqual(c.Columns, other.Columns)
}

// ForeignKey describes a referential constraint from one table to another.
// It is the only schema element that establishes directional dependency:
// the table owning the foreign key depends on RefTable.
type ForeignKey struct {
	Name       string   `json:"name" yaml:"name"`
	Columns    []string `json:"columns" yaml:"columns"`
	RefTable   string   `json:"refTable" yaml:"refTable"`
	RefColumns []string `json:"refColumns" yaml:"refColumns"`
	OnDelete   string   `json:"onDelete,omitempty" yaml:"onDelete,omitempty"`
	OnUpdate   string   `json:"onUpdate,omitempty" yaml:"onUpdate,omitempty"`
}

// Equal reports whether two foreign keys are structurally identical.
func (fk *ForeignKey) Equal(other *ForeignKey) bool {
	if fk == nil || other == nil {
		return fk == other
	}
	return fk.RefTable == other.RefTable &&
		strings.EqualFold(fk.OnDelete, other.OnDelete) &&
		strings.EqualFold(fk.OnUpdate, other.OnUpdate) &&
		slicesEqual(fk.Columns, other.Columns) &&
		slicesEqual(fk.RefColumns, other.RefColumns)
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
