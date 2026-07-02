package sqlite

import "github.com/guiiamorim/migrateit/internal/schema"

const (
	PrimaryKey schema.ConstraintType = "PRIMARY KEY"
	Unique     schema.ConstraintType = "UNIQUE"
	ForeignKey schema.ConstraintType = "FOREIGN KEY"
	Index      schema.ConstraintType = "INDEX"
)
