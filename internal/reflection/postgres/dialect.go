package postgres

import (
	"fmt"
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// Dialect renders schema changes as PostgreSQL DDL. It implements schema.Dialect.
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

func (d *Dialect) columnDef(c *schema.Column) string {
	var b strings.Builder
	b.WriteString(quote(c.Name))
	b.WriteString(" ")
	b.WriteString(c.Definition)
	if c.Extra != "" {
		b.WriteString(" ")
		b.WriteString(c.Extra)
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
	fmt.Fprintf(&b, "CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		quote(fk.Name), quoteList(fk.Columns), quote(fk.RefTable), quoteList(fk.RefColumns))
	if fk.OnDelete != "" {
		fmt.Fprintf(&b, " ON DELETE %s", fk.OnDelete)
	}
	if fk.OnUpdate != "" {
		fmt.Fprintf(&b, " ON UPDATE %s", fk.OnUpdate)
	}
	return b.String()
}

func (d *Dialect) tableConstraintDef(c *schema.Constraint) string {
	switch c.Type {
	case PrimaryKey:
		return fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)", quote(c.Name), quoteList(c.Columns))
	case Unique:
		return fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", quote(c.Name), quoteList(c.Columns))
	case Check:
		return fmt.Sprintf("CONSTRAINT %s CHECK (%s)", quote(c.Name), c.Expression)
	default:
		return ""
	}
}

func (d *Dialect) CreateTable(t *schema.Table, inlineFKs []*schema.ForeignKey) string {
	var defs []string
	for _, c := range t.Columns {
		defs = append(defs, "  "+d.columnDef(c))
	}
	if t.PrimaryKey != nil {
		defs = append(defs, "  "+d.tableConstraintDef(t.PrimaryKey))
	}
	// UNIQUE constraints are emitted inline; plain indexes become CREATE INDEX
	// statements afterwards (returned by the planner via AddConstraint), so they
	// are skipped here.
	for _, c := range t.Constraints {
		if c.Type == Unique {
			defs = append(defs, "  "+d.tableConstraintDef(c))
		}
	}
	for _, fk := range inlineFKs {
		defs = append(defs, "  "+d.foreignKeyDef(fk))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n%s\n)", quote(t.Name), strings.Join(defs, ",\n"))

	// Plain (non-unique) indexes on a freshly created table.
	for _, c := range t.Constraints {
		if c.Type == Index {
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

// ModifyColumn emits the granular ALTER COLUMN actions PostgreSQL requires.
// Because the dialect only sees the desired state, it always sets the type and
// nullability, and either sets or drops the default.
func (d *Dialect) ModifyColumn(table string, c *schema.Column) string {
	var actions []string
	actions = append(actions, fmt.Sprintf("ALTER COLUMN %s TYPE %s", quote(c.Name), c.Definition))
	if c.Nullable {
		actions = append(actions, fmt.Sprintf("ALTER COLUMN %s DROP NOT NULL", quote(c.Name)))
	} else {
		actions = append(actions, fmt.Sprintf("ALTER COLUMN %s SET NOT NULL", quote(c.Name)))
	}
	if c.Default != nil {
		actions = append(actions, fmt.Sprintf("ALTER COLUMN %s SET DEFAULT %s", quote(c.Name), *c.Default))
	} else {
		actions = append(actions, fmt.Sprintf("ALTER COLUMN %s DROP DEFAULT", quote(c.Name)))
	}
	return fmt.Sprintf("ALTER TABLE %s %s", quote(table), strings.Join(actions, ", "))
}

func (d *Dialect) AddConstraint(table string, c *schema.Constraint) string {
	switch c.Type {
	case PrimaryKey:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s PRIMARY KEY (%s)", quote(table), quote(c.Name), quoteList(c.Columns))
	case Unique:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s)", quote(table), quote(c.Name), quoteList(c.Columns))
	case Check:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s)", quote(table), quote(c.Name), c.Expression)
	default:
		return fmt.Sprintf("CREATE INDEX %s ON %s (%s)", quote(c.Name), quote(table), quoteList(c.Columns))
	}
}

func (d *Dialect) DropConstraint(table string, c *schema.Constraint) string {
	if c.Type == Index {
		return fmt.Sprintf("DROP INDEX %s", quote(c.Name))
	}
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", quote(table), quote(c.Name))
}

func (d *Dialect) AddForeignKey(table string, fk *schema.ForeignKey) string {
	return fmt.Sprintf("ALTER TABLE %s ADD %s", quote(table), d.foreignKeyDef(fk))
}

func (d *Dialect) DropForeignKey(table string, fk *schema.ForeignKey) string {
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", quote(table), quote(fk.Name))
}
