package schema

// Table is a database-agnostic description of a single table, including the
// elements needed to both diff it against another table and regenerate its DDL.
type Table struct {
	Name        string        `json:"name" yaml:"name"`
	Charset     string        `json:"charset,omitempty" yaml:"charset,omitempty"`
	Collation   string        `json:"collation,omitempty" yaml:"collation,omitempty"`
	Engine      string        `json:"engine,omitempty" yaml:"engine,omitempty"`
	Columns     []*Column     `json:"columns,omitempty" yaml:"columns,omitempty"`
	PrimaryKey  *Constraint   `json:"primaryKey,omitempty" yaml:"primaryKey,omitempty"`
	Constraints []*Constraint `json:"constraints,omitempty" yaml:"constraints,omitempty"` // unique keys, indexes, checks
	ForeignKeys []*ForeignKey `json:"foreignKeys,omitempty" yaml:"foreignKeys,omitempty"`
}

// Column returns the named column, or nil if the table has no such column.
func (t *Table) Column(name string) *Column {
	for _, c := range t.Columns {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Constraint returns the named constraint, or nil if absent.
func (t *Table) Constraint(name string) *Constraint {
	for _, c := range t.Constraints {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ForeignKey returns the named foreign key, or nil if absent.
func (t *Table) ForeignKey(name string) *ForeignKey {
	for _, fk := range t.ForeignKeys {
		if fk.Name == name {
			return fk
		}
	}
	return nil
}

// DependsOn returns the distinct set of tables this table references via
// foreign keys, excluding self-references.
func (t *Table) DependsOn() []string {
	seen := map[string]bool{}
	var deps []string
	for _, fk := range t.ForeignKeys {
		if fk.RefTable == t.Name || seen[fk.RefTable] {
			continue
		}
		seen[fk.RefTable] = true
		deps = append(deps, fk.RefTable)
	}
	return deps
}
