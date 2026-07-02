package mysql

import (
	"fmt"
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// Dialect renders schema changes as MySQL DDL. It implements schema.Dialect.
type Dialect struct{}

// NewDialect returns a MySQL dialect.
func NewDialect() *Dialect { return &Dialect{} }

var _ schema.Dialect = (*Dialect)(nil)

func quote(ident string) string {
	return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
}

func quoteList(idents []string) string {
	parts := make([]string, len(idents))
	for i, id := range idents {
		parts[i] = quote(id)
	}
	return strings.Join(parts, ", ")
}

// columnDef renders a single column definition, e.g.
// `id` bigint NOT NULL AUTO_INCREMENT.
func (d *Dialect) columnDef(c *schema.Column) string {
	var b strings.Builder
	b.WriteString(quote(c.Name))
	b.WriteString(" ")
	b.WriteString(c.Definition)

	if c.Nullable {
		b.WriteString(" NULL")
	} else {
		b.WriteString(" NOT NULL")
	}

	if c.Default != nil {
		b.WriteString(" DEFAULT ")
		b.WriteString(*c.Default)
	}

	if c.Extra != "" {
		b.WriteString(" ")
		b.WriteString(strings.ToUpper(c.Extra))
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

func (d *Dialect) constraintDef(c *schema.Constraint) string {
	switch c.Type {
	case PrimaryKey:
		return fmt.Sprintf("PRIMARY KEY (%s)", quoteList(c.Columns))
	case Unique:
		return fmt.Sprintf("UNIQUE KEY %s (%s)", quote(c.Name), quoteList(c.Columns))
	case Check:
		return fmt.Sprintf("CONSTRAINT %s CHECK (%s)", quote(c.Name), c.Expression)
	default: // plain index
		return fmt.Sprintf("KEY %s (%s)", quote(c.Name), quoteList(c.Columns))
	}
}

func (d *Dialect) CreateTable(t *schema.Table, inlineFKs []*schema.ForeignKey) string {
	var defs []string
	for _, c := range t.Columns {
		defs = append(defs, "  "+d.columnDef(c))
	}
	if t.PrimaryKey != nil {
		defs = append(defs, "  "+d.constraintDef(t.PrimaryKey))
	}
	for _, c := range t.Constraints {
		defs = append(defs, "  "+d.constraintDef(c))
	}
	for _, fk := range inlineFKs {
		defs = append(defs, "  "+d.foreignKeyDef(fk))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n%s\n)", quote(t.Name), strings.Join(defs, ",\n"))
	if t.Engine != "" {
		fmt.Fprintf(&b, " ENGINE=%s", t.Engine)
	}
	if t.Charset != "" {
		fmt.Fprintf(&b, " DEFAULT CHARSET=%s", t.Charset)
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
	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s", quote(table), d.columnDef(c))
}

func (d *Dialect) AddConstraint(table string, c *schema.Constraint) string {
	switch c.Type {
	case PrimaryKey:
		return fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY (%s)", quote(table), quoteList(c.Columns))
	case Unique:
		return fmt.Sprintf("ALTER TABLE %s ADD UNIQUE KEY %s (%s)", quote(table), quote(c.Name), quoteList(c.Columns))
	case Check:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s)", quote(table), quote(c.Name), c.Expression)
	default:
		return fmt.Sprintf("CREATE INDEX %s ON %s (%s)", quote(c.Name), quote(table), quoteList(c.Columns))
	}
}

func (d *Dialect) DropConstraint(table string, c *schema.Constraint) string {
	switch c.Type {
	case PrimaryKey:
		return fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY", quote(table))
	case Check:
		return fmt.Sprintf("ALTER TABLE %s DROP CHECK %s", quote(table), quote(c.Name))
	default: // unique keys and plain indexes are dropped as indexes
		return fmt.Sprintf("DROP INDEX %s ON %s", quote(c.Name), quote(table))
	}
}

func (d *Dialect) AddForeignKey(table string, fk *schema.ForeignKey) string {
	return fmt.Sprintf("ALTER TABLE %s ADD %s", quote(table), d.foreignKeyDef(fk))
}

func (d *Dialect) DropForeignKey(table string, fk *schema.ForeignKey) string {
	return fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", quote(table), quote(fk.Name))
}
