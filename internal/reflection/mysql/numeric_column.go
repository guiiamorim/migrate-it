package mysql

import (
	"github.com/guiiamorim/migrateit/internal/reflection"
	"slices"
)

type NumColumn struct {
	name        string
	columnType  reflection.ColumnType
	length      int
	precision   int
	scale       int
	unsigned    bool
	zerofill    bool
	autoInc     bool
	constraints []reflection.ConstraintSchema
}

func NewNumColumn(name string, columnType reflection.ColumnType) (*NumColumn, error) {
	if !slices.Contains(numericTypes(), columnType) {
		return nil, reflection.InvalidColumnTypeError{ColumnType: columnType}
	}

	return &NumColumn{
		name:       name,
		columnType: columnType,
	}, nil
}

func (c *NumColumn) SetLength(l int) *NumColumn {
	c.length = l
	return c
}

func (c *NumColumn) SetPrecision(p int) *NumColumn {
	c.precision = p
	return c
}

func (c *NumColumn) SetScale(s int) *NumColumn {
	c.scale = s
	return c
}

func (c *NumColumn) SetUnsigned() *NumColumn {
	c.unsigned = true
	return c
}

func (c *NumColumn) SetZerofill() *NumColumn {
	c.zerofill = true
	return c
}

func (c *NumColumn) AddConstraint(constraint reflection.ConstraintSchema) reflection.ColumnSchema {
	c.constraints = append(c.constraints, constraint)
	return c
}

func (c *NumColumn) Name() string {
	return c.name
}

func (c *NumColumn) Type() reflection.ColumnType {
	return c.columnType
}
