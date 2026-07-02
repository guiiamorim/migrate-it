package schema

import (
	"fmt"
	"sort"
)

// ChangeKind classifies a single schema change.
type ChangeKind int

const (
	CreateTable ChangeKind = iota
	DropTable
	AddColumn
	DropColumn
	ModifyColumn
	AddConstraint
	DropConstraint
	AddForeignKey
	DropForeignKey
)

func (k ChangeKind) String() string {
	switch k {
	case CreateTable:
		return "CREATE TABLE"
	case DropTable:
		return "DROP TABLE"
	case AddColumn:
		return "ADD COLUMN"
	case DropColumn:
		return "DROP COLUMN"
	case ModifyColumn:
		return "MODIFY COLUMN"
	case AddConstraint:
		return "ADD CONSTRAINT"
	case DropConstraint:
		return "DROP CONSTRAINT"
	case AddForeignKey:
		return "ADD FOREIGN KEY"
	case DropForeignKey:
		return "DROP FOREIGN KEY"
	default:
		return "UNKNOWN"
	}
}

// Change is one atomic difference between the source and target schemas.
// Only the fields relevant to Kind are populated.
type Change struct {
	Kind  ChangeKind
	Table string

	NewTable   *Table      // CreateTable
	Column     *Column     // Add/Modify column (the desired state)
	OldColumn  *Column     // ModifyColumn (the previous state)
	Constraint *Constraint // Add/DropConstraint
	ForeignKey *ForeignKey // Add/DropForeignKey
}

func (c Change) String() string {
	switch c.Kind {
	case CreateTable, DropTable:
		return fmt.Sprintf("%s %s", c.Kind, c.Table)
	case AddColumn, DropColumn, ModifyColumn:
		return fmt.Sprintf("%s %s.%s", c.Kind, c.Table, c.Column.Name)
	case AddConstraint, DropConstraint:
		return fmt.Sprintf("%s %s.%s", c.Kind, c.Table, c.Constraint.Name)
	case AddForeignKey, DropForeignKey:
		return fmt.Sprintf("%s %s.%s -> %s", c.Kind, c.Table, c.ForeignKey.Name, c.ForeignKey.RefTable)
	default:
		return c.Kind.String()
	}
}

// Diff is the full set of changes that turn the source schema into the target.
type Diff struct {
	Source  *Schema
	Target  *Schema
	Changes []Change
}

// Empty reports whether the two schemas are already equivalent.
func (d *Diff) Empty() bool {
	return len(d.Changes) == 0
}

// Compare computes the changes required to migrate `source` into `target`.
//
// The result is intentionally unordered with respect to dependencies; ordering
// (which respects the foreign-key graph) is the job of BuildMigration.
func Compare(source, target *Schema) *Diff {
	d := &Diff{Source: source, Target: target}

	// Tables only in target -> create. Tables only in source -> drop.
	for _, name := range target.TableNames() {
		if source.Table(name) == nil {
			d.Changes = append(d.Changes, Change{Kind: CreateTable, Table: name, NewTable: target.Table(name)})
		}
	}
	for _, name := range source.TableNames() {
		if target.Table(name) == nil {
			d.Changes = append(d.Changes, Change{Kind: DropTable, Table: name})
		}
	}

	// Tables in both -> compare contents.
	for _, name := range target.TableNames() {
		src := source.Table(name)
		if src == nil {
			continue
		}
		d.Changes = append(d.Changes, diffTable(src, target.Table(name))...)
	}

	return d
}

func diffTable(src, tgt *Table) []Change {
	var changes []Change

	// Columns.
	for _, col := range tgt.Columns {
		old := src.Column(col.Name)
		switch {
		case old == nil:
			changes = append(changes, Change{Kind: AddColumn, Table: tgt.Name, Column: col})
		case !old.Equal(col):
			changes = append(changes, Change{Kind: ModifyColumn, Table: tgt.Name, Column: col, OldColumn: old})
		}
	}
	for _, col := range src.Columns {
		if tgt.Column(col.Name) == nil {
			changes = append(changes, Change{Kind: DropColumn, Table: tgt.Name, Column: col})
		}
	}

	// Constraints (primary key, unique, index, check).
	changes = append(changes, diffConstraints(src, tgt)...)

	// Foreign keys.
	changes = append(changes, diffForeignKeys(src, tgt)...)

	return changes
}

func diffConstraints(src, tgt *Table) []Change {
	var changes []Change

	srcCons := constraintMap(src)
	tgtCons := constraintMap(tgt)

	for _, name := range sortedKeys(tgtCons) {
		want := tgtCons[name]
		have, ok := srcCons[name]
		if !ok {
			changes = append(changes, Change{Kind: AddConstraint, Table: tgt.Name, Constraint: want})
		} else if !have.Equal(want) {
			// A changed constraint is dropped then re-added.
			changes = append(changes,
				Change{Kind: DropConstraint, Table: tgt.Name, Constraint: have},
				Change{Kind: AddConstraint, Table: tgt.Name, Constraint: want},
			)
		}
	}
	for _, name := range sortedKeys(srcCons) {
		if _, ok := tgtCons[name]; !ok {
			changes = append(changes, Change{Kind: DropConstraint, Table: tgt.Name, Constraint: srcCons[name]})
		}
	}
	return changes
}

func diffForeignKeys(src, tgt *Table) []Change {
	var changes []Change

	srcFKs := foreignKeyMap(src)
	tgtFKs := foreignKeyMap(tgt)

	for _, name := range sortedKeys(tgtFKs) {
		want := tgtFKs[name]
		have, ok := srcFKs[name]
		if !ok {
			changes = append(changes, Change{Kind: AddForeignKey, Table: tgt.Name, ForeignKey: want})
		} else if !have.Equal(want) {
			changes = append(changes,
				Change{Kind: DropForeignKey, Table: tgt.Name, ForeignKey: have},
				Change{Kind: AddForeignKey, Table: tgt.Name, ForeignKey: want},
			)
		}
	}
	for _, name := range sortedKeys(srcFKs) {
		if _, ok := tgtFKs[name]; !ok {
			changes = append(changes, Change{Kind: DropForeignKey, Table: tgt.Name, ForeignKey: srcFKs[name]})
		}
	}
	return changes
}

func constraintMap(t *Table) map[string]*Constraint {
	m := map[string]*Constraint{}
	if t.PrimaryKey != nil {
		m[t.PrimaryKey.Name] = t.PrimaryKey
	}
	for _, c := range t.Constraints {
		m[c.Name] = c
	}
	return m
}

func foreignKeyMap(t *Table) map[string]*ForeignKey {
	m := map[string]*ForeignKey{}
	for _, fk := range t.ForeignKeys {
		m[fk.Name] = fk
	}
	return m
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
