package sqlite

import (
	"fmt"
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// Dialect renders schema changes as SQLite DDL. It implements schema.Dialect.
//
// SQLite's ALTER TABLE only supports ADD COLUMN, DROP COLUMN and RENAME. Changes
// that other engines express with ALTER (modifying a column, adding/dropping a
// primary key, or adding/dropping a foreign key) require rebuilding the table.
// Rather than silently emitting invalid SQL, those operations return an
// explanatory SQL comment so the generated script is honest about what it cannot
// express. Index-backed changes (unique and plain indexes) are fully supported.
type Dialect struct{}

func NewDialect() *Dialect { return &Dialect{} }

var _ schema.Dialect = (*Dialect)(nil)

func quote(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func quoteList(idents []string) string {
	parts := make([]string, len(idents))
	for i, id := range idents {
		parts[i] = quote(id)
	}
	return strings.Join(parts, ", ")
}

func unsupported(format string, args ...any) string {
	return "-- migrate-it: " + fmt.Sprintf(format, args...) + " (requires a table rebuild in SQLite; do this manually)"
}

func (d *Dialect) columnDef(c *schema.Column) string {
	var b strings.Builder
	b.WriteString(quote(c.Name))
	if c.Definition != "" {
		b.WriteString(" ")
		b.WriteString(c.Definition)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	if c.Default != nil {
		b.WriteString(" DEFAULT ")
		b.WriteString(*c.Default)
	}
	return b.String()
}

func (d *Dialect) foreignKeyDef(fk *schema.ForeignKey) string {
	var b strings.Builder
	fmt.Fprintf(&b, "FOREIGN KEY (%s) REFERENCES %s (%s)",
		quoteList(fk.Columns), quote(fk.RefTable), quoteList(fk.RefColumns))
	if fk.OnDelete != "" {
		fmt.Fprintf(&b, " ON DELETE %s", fk.OnDelete)
	}
	if fk.OnUpdate != "" {
		fmt.Fprintf(&b, " ON UPDATE %s", fk.OnUpdate)
	}
	return b.String()
}

func (d *Dialect) CreateTable(t *schema.Table, inlineFKs []*schema.ForeignKey) string {
	var defs []string
	for _, c := range t.Columns {
		defs = append(defs, "  "+d.columnDef(c))
	}
	if t.PrimaryKey != nil {
		defs = append(defs, fmt.Sprintf("  PRIMARY KEY (%s)", quoteList(t.PrimaryKey.Columns)))
	}
	for _, fk := range inlineFKs {
		defs = append(defs, "  "+d.foreignKeyDef(fk))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n%s\n)", quote(t.Name), strings.Join(defs, ",\n"))

	// Unique and plain indexes become separate CREATE INDEX statements; SQLite
	// implements unique constraints as unique indexes anyway.
	for _, c := range t.Constraints {
		if c.Type == Unique || c.Type == Index {
			b.WriteString(";\n")
			b.WriteString(d.AddConstraint(t.Name, c))
		}
	}
	return b.String()
}

func (d *Dialect) DropTable(table string) string {
	return fmt.Sprintf("DROP TABLE %s", quote(table))
}

func (d *Dialect) AddColumn(table string, c *schema.Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", quote(table), d.columnDef(c))
}

func (d *Dialect) DropColumn(table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quote(table), quote(column))
}

func (d *Dialect) ModifyColumn(table string, c *schema.Column) string {
	return unsupported("cannot modify column %s.%s", table, c.Name)
}

func (d *Dialect) AddConstraint(table string, c *schema.Constraint) string {
	switch c.Type {
	case Unique:
		return fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", quote(c.Name), quote(table), quoteList(c.Columns))
	case Index:
		return fmt.Sprintf("CREATE INDEX %s ON %s (%s)", quote(c.Name), quote(table), quoteList(c.Columns))
	default: // primary key / check
		return unsupported("cannot add %s constraint %s on %s", c.Type, c.Name, table)
	}
}

func (d *Dialect) DropConstraint(table string, c *schema.Constraint) string {
	switch c.Type {
	case Unique, Index:
		return fmt.Sprintf("DROP INDEX %s", quote(c.Name))
	default:
		return unsupported("cannot drop %s constraint %s on %s", c.Type, c.Name, table)
	}
}

func (d *Dialect) AddForeignKey(table string, fk *schema.ForeignKey) string {
	return unsupported("cannot add foreign key %s on %s", fk.Name, table)
}

func (d *Dialect) DropForeignKey(table string, fk *schema.ForeignKey) string {
	return unsupported("cannot drop foreign key %s on %s", fk.Name, table)
}
