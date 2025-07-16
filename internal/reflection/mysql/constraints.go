package mysql

import (
	"github.com/guiiamorim/migrateit/internal/reflection"
)

const (
	PrimaryKey reflection.ConstraintType = "PRIMARY KEY"
	Unique     reflection.ConstraintType = "UNIQUE"
	NotNull    reflection.ConstraintType = "NOT NULL"
	ForeignKey reflection.ConstraintType = "FOREIGN KEY"
	Check      reflection.ConstraintType = "CHECK"
	Default    reflection.ConstraintType = "DEFAULT"
	Index      reflection.ConstraintType = "CREATE INDEX"
)
