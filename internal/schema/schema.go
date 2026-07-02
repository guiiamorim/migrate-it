package schema

import "sort"

// Schema is a reflected snapshot of a single database: its name and the set of
// tables it contains, keyed by table name.
type Schema struct {
	Name   string
	Tables map[string]*Table
}

// New returns an empty schema with the given name.
func New(name string) *Schema {
	return &Schema{Name: name, Tables: map[string]*Table{}}
}

// AddTable inserts or replaces a table and returns the schema for chaining.
func (s *Schema) AddTable(t *Table) *Schema {
	if s.Tables == nil {
		s.Tables = map[string]*Table{}
	}
	s.Tables[t.Name] = t
	return s
}

// Table returns the named table, or nil if absent.
func (s *Schema) Table(name string) *Table {
	return s.Tables[name]
}

// TableNames returns all table names in deterministic (sorted) order.
func (s *Schema) TableNames() []string {
	names := make([]string, 0, len(s.Tables))
	for name := range s.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
