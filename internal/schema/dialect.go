package schema

// Dialect renders schema changes as driver-specific SQL. Each method returns a
// complete statement without a trailing semicolon; the migration planner adds
// separators and ordering.
//
// CreateTable receives the foreign keys that should be defined inline. The
// planner passes only those whose referenced table already exists at that point
// in the ordering; foreign keys that would point "forward" (into a dependency
// cycle) are emitted separately via AddForeignKey.
type Dialect interface {
	CreateTable(t *Table, inlineFKs []*ForeignKey) string
	DropTable(table string) string

	AddColumn(table string, c *Column) string
	DropColumn(table, column string) string
	ModifyColumn(table string, c *Column) string

	AddConstraint(table string, c *Constraint) string
	DropConstraint(table string, c *Constraint) string

	AddForeignKey(table string, fk *ForeignKey) string
	DropForeignKey(table string, fk *ForeignKey) string
}
