package reflection

type ColumnSchema interface {
	AddConstraint(constraint ConstraintSchema) ColumnSchema
}

type ColumnType string

type InvalidColumnTypeError struct {
	ColumnType ColumnType
}

func (e InvalidColumnTypeError) Error() string {
	return "invalid column type: " + string(e.ColumnType)
}
