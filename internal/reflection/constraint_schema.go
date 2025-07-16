package reflection

type ConstraintSchema interface {
	Type() ConstraintType
	SQL() string
}

type ConstraintType string
