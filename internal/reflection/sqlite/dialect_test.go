package sqlite

import (
	"strings"
	"testing"

	"github.com/guiiamorim/migrateit/internal/schema"
)

func TestDialect_CreateTableInlinesPKAndFK(t *testing.T) {
	d := NewDialect()
	tbl := &schema.Table{
		Name: "orders",
		Columns: []*schema.Column{
			{Name: "id", Definition: "INTEGER"},
			{Name: "user_id", Definition: "INTEGER", Nullable: false},
		},
		PrimaryKey: &schema.Constraint{Name: "orders_pk", Type: PrimaryKey, Columns: []string{"id"}},
	}
	fk := &schema.ForeignKey{Name: "fk_orders_user_id", Columns: []string{"user_id"},
		RefTable: "users", RefColumns: []string{"id"}, OnDelete: "CASCADE"}

	got := d.CreateTable(tbl, []*schema.ForeignKey{fk})
	wants := []string{
		`CREATE TABLE "orders"`,
		`PRIMARY KEY ("id")`,
		`FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q\n--- got ---\n%s", w, got)
		}
	}
}

func TestDialect_UniqueConstraintBecomesUniqueIndex(t *testing.T) {
	got := NewDialect().AddConstraint("users", &schema.Constraint{Name: "ux_email", Type: Unique, Columns: []string{"email"}})
	if got != `CREATE UNIQUE INDEX "ux_email" ON "users" ("email")` {
		t.Errorf("unexpected: %s", got)
	}
}

func TestDialect_UnsupportedOperationsAreCommented(t *testing.T) {
	d := NewDialect()
	stmts := []string{
		d.ModifyColumn("users", &schema.Column{Name: "x", Definition: "TEXT"}),
		d.AddForeignKey("orders", &schema.ForeignKey{Name: "fk"}),
		d.DropForeignKey("orders", &schema.ForeignKey{Name: "fk"}),
	}
	for _, s := range stmts {
		if !strings.HasPrefix(s, "--") {
			t.Errorf("expected an explanatory comment, got executable SQL: %s", s)
		}
	}
}

func TestColumnType_Affinity(t *testing.T) {
	cases := map[string]schema.ColumnType{
		"INTEGER":      Integer,
		"VARCHAR(255)": Text,
		"":             Blob,
		"REAL":         Real,
		"DECIMAL(5,2)": Numeric,
	}
	for in, want := range cases {
		if got := columnType(in); got != want {
			t.Errorf("columnType(%q) = %s, want %s", in, got, want)
		}
	}
}
