package mysql

import (
	"github.com/guiiamorim/migrateit/internal/schema"
)

const (
	PrimaryKey schema.ConstraintType = "PRIMARY KEY"
	Unique     schema.ConstraintType = "UNIQUE"
	NotNull    schema.ConstraintType = "NOT NULL"
	ForeignKey schema.ConstraintType = "FOREIGN KEY"
	Check      schema.ConstraintType = "CHECK"
	Default    schema.ConstraintType = "DEFAULT"
	Index      schema.ConstraintType = "INDEX"
)
