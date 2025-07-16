package reflection

type TableSchema interface {
	AddColumn(column ColumnSchema) TableSchema
}
