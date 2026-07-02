package sqlite

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/guiiamorim/migrateit/internal/schema"
	_ "modernc.org/sqlite"
)

// openTestDB creates a throwaway SQLite database seeded with the given DDL.
func openTestDB(t *testing.T, ddl ...string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
	return db
}

func TestReflect_ColumnsPrimaryKeyAndDefault(t *testing.T) {
	db := openTestDB(t,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			status INTEGER DEFAULT 1
		)`,
	)
	s, err := NewReflector(db, "test").Reflect()
	if err != nil {
		t.Fatalf("reflect: %v", err)
	}

	users := s.Table("users")
	if users == nil {
		t.Fatal("users table not reflected")
	}
	if len(users.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(users.Columns))
	}
	if users.PrimaryKey == nil || len(users.PrimaryKey.Columns) != 1 || users.PrimaryKey.Columns[0] != "id" {
		t.Errorf("primary key not reflected correctly: %+v", users.PrimaryKey)
	}
	email := users.Column("email")
	if email == nil || email.Nullable {
		t.Errorf("email should be NOT NULL: %+v", email)
	}
	status := users.Column("status")
	if status == nil || status.Default == nil || *status.Default != "1" {
		t.Errorf("status default not reflected: %+v", status)
	}
}

func TestReflect_ForeignKeysAndIndexes(t *testing.T) {
	db := openTestDB(t,
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX ux_users_email ON users (email)`,
		`CREATE TABLE orders (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
	)
	s, err := NewReflector(db, "test").Reflect()
	if err != nil {
		t.Fatalf("reflect: %v", err)
	}

	users := s.Table("users")
	var unique *string
	for _, c := range users.Constraints {
		if c.Type == Unique && c.Name == "ux_users_email" {
			n := c.Name
			unique = &n
		}
	}
	if unique == nil {
		t.Errorf("unique index not reflected: %+v", users.Constraints)
	}

	orders := s.Table("orders")
	if len(orders.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(orders.ForeignKeys))
	}
	fk := orders.ForeignKeys[0]
	if fk.RefTable != "users" || fk.Columns[0] != "user_id" || fk.RefColumns[0] != "id" {
		t.Errorf("foreign key reflected incorrectly: %+v", fk)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("expected ON DELETE CASCADE, got %q", fk.OnDelete)
	}
	// orders depends on users -> the dependency must surface in the graph.
	if deps := orders.DependsOn(); len(deps) != 1 || deps[0] != "users" {
		t.Errorf("expected orders to depend on users, got %v", deps)
	}
}

func TestReflect_ForeignKeyNamesAreStable(t *testing.T) {
	// The same logical schema with foreign keys declared in opposite order must
	// reflect to identical foreign-key names (regression test for PRAGMA id churn).
	ddlA := []string{
		`CREATE TABLE a (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE b (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE c (
			id INTEGER PRIMARY KEY, a_id INTEGER, b_id INTEGER,
			FOREIGN KEY (a_id) REFERENCES a(id),
			FOREIGN KEY (b_id) REFERENCES b(id)
		)`,
	}
	ddlB := []string{
		`CREATE TABLE a (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE b (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE c (
			id INTEGER PRIMARY KEY, a_id INTEGER, b_id INTEGER,
			FOREIGN KEY (b_id) REFERENCES b(id),
			FOREIGN KEY (a_id) REFERENCES a(id)
		)`,
	}
	sA, err := NewReflector(openTestDB(t, ddlA...), "a").Reflect()
	if err != nil {
		t.Fatal(err)
	}
	sB, err := NewReflector(openTestDB(t, ddlB...), "b").Reflect()
	if err != nil {
		t.Fatal(err)
	}

	got := fkNames(sA.Table("c"))
	want := fkNames(sB.Table("c"))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("foreign key names differ across declaration order:\n A: %v\n B: %v", got, want)
	}
}

func fkNames(t *schema.Table) []string {
	names := make([]string, 0, len(t.ForeignKeys))
	for _, fk := range t.ForeignKeys {
		names = append(names, fk.Name)
	}
	sort.Strings(names)
	return names
}
