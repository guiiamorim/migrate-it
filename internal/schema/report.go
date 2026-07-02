package schema

// DiffReport is a serialization-friendly view of a Diff, suitable for encoding
// as JSON or YAML. Unlike Diff itself it does not embed the full source/target
// schemas (only their names), and it renders each change's kind as a string.
type DiffReport struct {
	Source  string         `json:"source" yaml:"source"`
	Target  string         `json:"target" yaml:"target"`
	Changes []ChangeReport `json:"changes" yaml:"changes"`
}

// ChangeReport is the serializable form of a single Change. Only the fields
// relevant to the change kind are populated.
type ChangeReport struct {
	Kind       string      `json:"kind" yaml:"kind"`
	Table      string      `json:"table" yaml:"table"`
	NewTable   *Table      `json:"newTable,omitempty" yaml:"newTable,omitempty"`
	Column     *Column     `json:"column,omitempty" yaml:"column,omitempty"`
	OldColumn  *Column     `json:"oldColumn,omitempty" yaml:"oldColumn,omitempty"`
	Constraint *Constraint `json:"constraint,omitempty" yaml:"constraint,omitempty"`
	ForeignKey *ForeignKey `json:"foreignKey,omitempty" yaml:"foreignKey,omitempty"`
}

// Report builds a serializable view of the diff. The change list preserves the
// order produced by Compare (grouped by table, not dependency-ordered — that is
// the concern of BuildMigration, and only meaningful for SQL output).
func (d *Diff) Report() DiffReport {
	r := DiffReport{Source: d.Source.Name, Target: d.Target.Name, Changes: []ChangeReport{}}
	for _, c := range d.Changes {
		r.Changes = append(r.Changes, ChangeReport{
			Kind:       c.Kind.String(),
			Table:      c.Table,
			NewTable:   c.NewTable,
			Column:     c.Column,
			OldColumn:  c.OldColumn,
			Constraint: c.Constraint,
			ForeignKey: c.ForeignKey,
		})
	}
	return r
}
