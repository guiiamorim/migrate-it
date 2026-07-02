package schema

import (
	"fmt"
	"strings"
	"testing"
)

// recordDialect emits terse, greppable statements so tests can assert on
// ordering without depending on a real SQL grammar.
type recordDialect struct{}

func (recordDialect) CreateTable(t *Table, inlineFKs []*ForeignKey) string {
	names := make([]string, len(inlineFKs))
	for i, fk := range inlineFKs {
		names[i] = fk.Name
	}
	return fmt.Sprintf("CREATE %s [fks:%s]", t.Name, strings.Join(names, ","))
}
func (recordDialect) DropTable(table string) string { return "DROP " + table }
func (recordDialect) AddColumn(table string, c *Column) string {
	return fmt.Sprintf("ADDCOL %s.%s", table, c.Name)
}
func (recordDialect) DropColumn(table, column string) string {
	return fmt.Sprintf("DROPCOL %s.%s", table, column)
}
func (recordDialect) ModifyColumn(table string, c *Column) string {
	return fmt.Sprintf("MODCOL %s.%s", table, c.Name)
}
func (recordDialect) AddConstraint(table string, c *Constraint) string {
	return fmt.Sprintf("ADDCON %s.%s", table, c.Name)
}
func (recordDialect) DropConstraint(table string, c *Constraint) string {
	return fmt.Sprintf("DROPCON %s.%s", table, c.Name)
}
func (recordDialect) AddForeignKey(table string, fk *ForeignKey) string {
	return fmt.Sprintf("ADDFK %s.%s->%s", table, fk.Name, fk.RefTable)
}
func (recordDialect) DropForeignKey(table string, fk *ForeignKey) string {
	return fmt.Sprintf("DROPFK %s.%s", table, fk.Name)
}

func indexOfStmt(stmts []string, substr string) int {
	for i, s := range stmts {
		if strings.Contains(s, substr) {
			return i
		}
	}
	return -1
}

func TestBuildMigration_CreatesParentsBeforeChildren(t *testing.T) {
	src := New("db")
	tgt := New("db").
		AddTable(&Table{Name: "users"}).
		AddTable(tableWithFKs("orders", fk("fk_orders_user", "users")))

	m := BuildMigration(Compare(src, tgt), recordDialect{})

	users := indexOfStmt(m.Statements, "CREATE users")
	orders := indexOfStmt(m.Statements, "CREATE orders")
	if users == -1 || orders == -1 {
		t.Fatalf("missing create statements: %v", m.Statements)
	}
	if users > orders {
		t.Errorf("parent table must be created first:\n%s", strings.Join(m.Statements, "\n"))
	}
	// The FK references an existing (just-created) table, so it should be inlined.
	if idx := indexOfStmt(m.Statements, "CREATE orders [fks:fk_orders_user]"); idx == -1 {
		t.Errorf("expected fk inlined into orders create:\n%s", strings.Join(m.Statements, "\n"))
	}
}

func TestBuildMigration_DropsChildrenBeforeParents(t *testing.T) {
	src := New("db").
		AddTable(&Table{Name: "users"}).
		AddTable(tableWithFKs("orders", fk("fk_orders_user", "users")))
	tgt := New("db")

	m := BuildMigration(Compare(src, tgt), recordDialect{})

	// FK must be dropped before the tables it ties together.
	dropFK := indexOfStmt(m.Statements, "DROPFK orders.fk_orders_user")
	dropOrders := indexOfStmt(m.Statements, "DROP orders")
	dropUsers := indexOfStmt(m.Statements, "DROP users")
	if dropFK == -1 || dropOrders == -1 || dropUsers == -1 {
		t.Fatalf("missing drop statements: %v", m.Statements)
	}
	if !(dropFK < dropOrders && dropOrders < dropUsers) {
		t.Errorf("expected drop order fk < child < parent:\n%s", strings.Join(m.Statements, "\n"))
	}
}

func TestBuildMigration_CyclicForeignKeysDeferred(t *testing.T) {
	// a <-> b mutual references. Both tables are new.
	src := New("db")
	tgt := New("db").
		AddTable(tableWithFKs("a", fk("fk_a_b", "b"))).
		AddTable(tableWithFKs("b", fk("fk_b_a", "a")))

	m := BuildMigration(Compare(src, tgt), recordDialect{})

	createA := indexOfStmt(m.Statements, "CREATE a")
	createB := indexOfStmt(m.Statements, "CREATE b")
	addFKa := indexOfStmt(m.Statements, "ADDFK a.fk_a_b")
	addFKb := indexOfStmt(m.Statements, "ADDFK b.fk_b_a")

	if createA == -1 || createB == -1 {
		t.Fatalf("both tables must be created: %v", m.Statements)
	}
	// At least one cyclic FK must be deferred to an ADDFK after both creates.
	if addFKa == -1 && addFKb == -1 {
		t.Fatalf("expected at least one deferred FK:\n%s", strings.Join(m.Statements, "\n"))
	}
	for _, fkIdx := range []int{addFKa, addFKb} {
		if fkIdx != -1 && (fkIdx < createA || fkIdx < createB) {
			t.Errorf("deferred FK must come after both creates:\n%s", strings.Join(m.Statements, "\n"))
		}
	}
}

func TestBuildMigration_AddForeignKeyToExistingTablesComesLast(t *testing.T) {
	src := New("db").
		AddTable(&Table{Name: "users"}).
		AddTable(&Table{Name: "orders"})
	tgt := New("db").
		AddTable(&Table{Name: "users"}).
		AddTable(tableWithFKs("orders", fk("fk_orders_user", "users")))

	m := BuildMigration(Compare(src, tgt), recordDialect{})
	if got := indexOfStmt(m.Statements, "ADDFK orders.fk_orders_user->users"); got == -1 {
		t.Fatalf("expected an ADD FOREIGN KEY statement:\n%s", strings.Join(m.Statements, "\n"))
	}
}
