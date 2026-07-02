package schema

import "strings"

// Migration is an ordered sequence of SQL statements that, applied in order,
// transforms the source schema into the target schema.
type Migration struct {
	Statements []string
}

// SQL joins the statements into an executable script.
func (m *Migration) SQL() string {
	var b strings.Builder
	for _, s := range m.Statements {
		b.WriteString(s)
		b.WriteString(";\n")
	}
	return b.String()
}

// BuildMigration turns a diff into ordered SQL using the foreign-key graph to
// satisfy inter-table precedence. The ordering guarantees that:
//
//  1. Foreign keys are dropped before the columns/tables they touch.
//  2. Tables are dropped children-first (reverse dependency order).
//  3. New tables are created parents-first (dependency order), with foreign
//     keys inlined only when their referenced table already exists.
//  4. Existing tables are altered (columns, then constraints).
//  5. Remaining foreign keys are added last, once every referenced table and
//     column is guaranteed to exist. This also resolves dependency cycles.
func BuildMigration(d *Diff, dialect Dialect) *Migration {
	m := &Migration{}

	byTable := groupChanges(d.Changes)

	sourceGraph := BuildGraph(d.Source)
	targetGraph := BuildGraph(d.Target)

	dropping := changeSet(d.Changes, DropTable)

	// (1) Drop foreign keys first so later drops/alters are unobstructed. This
	// covers both FKs removed from surviving tables (DropForeignKey changes) and
	// every FK owned by a table that is about to be dropped, so the order in
	// which tables disappear can never be blocked by a dangling constraint.
	for _, table := range d.Source.TableNames() {
		if dropping[table] {
			for _, fk := range d.Source.Table(table).ForeignKeys {
				m.add(dialect.DropForeignKey(table, fk))
			}
			continue
		}
		for _, c := range byTable[table] {
			if c.Kind == DropForeignKey {
				m.add(dialect.DropForeignKey(c.Table, c.ForeignKey))
			}
		}
	}

	// (2) Drop tables children-first.
	for _, table := range sourceGraph.DropOrder() {
		if dropping[table] {
			m.add(dialect.DropTable(table))
		}
	}

	// (3) Create new tables parents-first, inlining only safe foreign keys.
	creating := changeSet(d.Changes, CreateTable)
	existing := map[string]bool{} // tables that exist at this point in the script
	for name := range d.Source.Tables {
		existing[name] = true
	}
	for table := range dropping {
		delete(existing, table)
	}
	var deferredFKs []Change
	for _, table := range targetGraph.CreationOrder() {
		if !creating[table] {
			continue
		}
		t := d.Target.Table(table)
		var inline []*ForeignKey
		for _, fk := range t.ForeignKeys {
			if fk.RefTable == table || existing[fk.RefTable] {
				inline = append(inline, fk)
			} else {
				deferredFKs = append(deferredFKs, Change{Kind: AddForeignKey, Table: table, ForeignKey: fk})
			}
		}
		m.add(dialect.CreateTable(t, inline))
		existing[table] = true
	}

	// (4) Alter existing tables: columns first, then constraints.
	for _, table := range d.Target.TableNames() {
		if creating[table] {
			continue
		}
		for _, c := range byTable[table] {
			switch c.Kind {
			case AddColumn:
				m.add(dialect.AddColumn(c.Table, c.Column))
			case ModifyColumn:
				m.add(dialect.ModifyColumn(c.Table, c.Column))
			case DropColumn:
				m.add(dialect.DropColumn(c.Table, c.Column.Name))
			}
		}
		for _, c := range byTable[table] {
			switch c.Kind {
			case DropConstraint:
				m.add(dialect.DropConstraint(c.Table, c.Constraint))
			case AddConstraint:
				m.add(dialect.AddConstraint(c.Table, c.Constraint))
			}
		}
	}

	// (5) Add foreign keys last (deferred cyclic ones, plus any on existing
	// tables), now that all referenced tables and columns exist.
	for _, table := range d.Target.TableNames() {
		for _, c := range byTable[table] {
			if c.Kind == AddForeignKey {
				m.add(dialect.AddForeignKey(c.Table, c.ForeignKey))
			}
		}
	}
	for _, c := range deferredFKs {
		m.add(dialect.AddForeignKey(c.Table, c.ForeignKey))
	}

	return m
}

func (m *Migration) add(stmt string) {
	if stmt != "" {
		m.Statements = append(m.Statements, stmt)
	}
}

// groupChanges buckets changes by table, preserving their relative order.
func groupChanges(changes []Change) map[string][]Change {
	byTable := map[string][]Change{}
	for _, c := range changes {
		byTable[c.Table] = append(byTable[c.Table], c)
	}
	return byTable
}

func changeSet(changes []Change, kind ChangeKind) map[string]bool {
	set := map[string]bool{}
	for _, c := range changes {
		if c.Kind == kind {
			set[c.Table] = true
		}
	}
	return set
}
