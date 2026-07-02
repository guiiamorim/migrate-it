package postgres

import "github.com/guiiamorim/migrateit/internal/schema"

const (
	PrimaryKey schema.ConstraintType = "PRIMARY KEY"
	Unique     schema.ConstraintType = "UNIQUE"
	ForeignKey schema.ConstraintType = "FOREIGN KEY"
	Check      schema.ConstraintType = "CHECK"
	Index      schema.ConstraintType = "INDEX"
)

// fkAction maps a pg_constraint confdeltype/confupdtype code to its SQL clause.
func fkAction(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return ""
	}
}
