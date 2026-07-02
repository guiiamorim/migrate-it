package schema

import "testing"

func col(name, def string, nullable bool) *Column {
	return &Column{Name: name, Definition: def, Nullable: nullable, Type: ColumnType(def)}
}

func countKind(d *Diff, kind ChangeKind) int {
	n := 0
	for _, c := range d.Changes {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

func TestCompare_IdenticalSchemas(t *testing.T) {
	build := func() *Schema {
		return New("db").AddTable(&Table{
			Name:    "users",
			Columns: []*Column{col("id", "bigint", false), col("name", "varchar(255)", false)},
		})
	}
	d := Compare(build(), build())
	if !d.Empty() {
		t.Fatalf("expected no changes, got %v", d.Changes)
	}
}

func TestCompare_AddAndDropTable(t *testing.T) {
	src := New("db").AddTable(&Table{Name: "old"})
	tgt := New("db").AddTable(&Table{Name: "new"})

	d := Compare(src, tgt)
	if countKind(d, CreateTable) != 1 {
		t.Errorf("expected 1 CreateTable, got %d", countKind(d, CreateTable))
	}
	if countKind(d, DropTable) != 1 {
		t.Errorf("expected 1 DropTable, got %d", countKind(d, DropTable))
	}
}

func TestCompare_ColumnChanges(t *testing.T) {
	src := New("db").AddTable(&Table{
		Name:    "users",
		Columns: []*Column{col("id", "bigint", false), col("legacy", "text", true)},
	})
	tgt := New("db").AddTable(&Table{
		Name: "users",
		Columns: []*Column{
			col("id", "bigint", false),
			col("email", "varchar(255)", false), // added
		},
	})

	d := Compare(src, tgt)
	if countKind(d, AddColumn) != 1 {
		t.Errorf("expected 1 AddColumn, got %d", countKind(d, AddColumn))
	}
	if countKind(d, DropColumn) != 1 {
		t.Errorf("expected 1 DropColumn, got %d", countKind(d, DropColumn))
	}
}

func TestCompare_ModifyColumn(t *testing.T) {
	src := New("db").AddTable(&Table{Name: "users", Columns: []*Column{col("name", "varchar(100)", false)}})
	tgt := New("db").AddTable(&Table{Name: "users", Columns: []*Column{col("name", "varchar(255)", false)}})

	d := Compare(src, tgt)
	if countKind(d, ModifyColumn) != 1 {
		t.Fatalf("expected 1 ModifyColumn, got %d (%v)", countKind(d, ModifyColumn), d.Changes)
	}
}

func TestCompare_ForeignKeyChange(t *testing.T) {
	src := New("db").AddTable(&Table{Name: "orders"})
	tgt := New("db").AddTable(&Table{
		Name:        "orders",
		ForeignKeys: []*ForeignKey{{Name: "fk_user", RefTable: "users", Columns: []string{"user_id"}, RefColumns: []string{"id"}}},
	})

	d := Compare(src, tgt)
	if countKind(d, AddForeignKey) != 1 {
		t.Errorf("expected 1 AddForeignKey, got %d", countKind(d, AddForeignKey))
	}
}

func TestColumnEqual_DefaultPointer(t *testing.T) {
	def := "0"
	a := &Column{Name: "n", Definition: "int", Default: &def}
	b := &Column{Name: "n", Definition: "int", Default: &def}
	if !a.Equal(b) {
		t.Error("columns with equal defaults should be equal")
	}
	c := &Column{Name: "n", Definition: "int"} // nil default
	if a.Equal(c) {
		t.Error("column with default must differ from column without")
	}
}
