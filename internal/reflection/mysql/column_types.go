package mysql

import (
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

const (
	Int        schema.ColumnType = "INT"
	Varchar    schema.ColumnType = "VARCHAR"
	Text       schema.ColumnType = "TEXT"
	Longtext   schema.ColumnType = "LONGTEXT"
	Mediumtext schema.ColumnType = "MEDIUMTEXT"
	Tinytext   schema.ColumnType = "TINYTEXT"
	Date       schema.ColumnType = "DATE"
	Datetime   schema.ColumnType = "DATETIME"
	Timestamp  schema.ColumnType = "TIMESTAMP"
	Decimal    schema.ColumnType = "DECIMAL"
	Float      schema.ColumnType = "FLOAT"
	Double     schema.ColumnType = "DOUBLE"
	Enum       schema.ColumnType = "ENUM"
	Set        schema.ColumnType = "SET"
	Tinyint    schema.ColumnType = "TINYINT"
	Smallint   schema.ColumnType = "SMALLINT"
	Mediumint  schema.ColumnType = "MEDIUMINT"
	Bigint     schema.ColumnType = "BIGINT"
	Char       schema.ColumnType = "CHAR"
	Blob       schema.ColumnType = "BLOB"
	Longblob   schema.ColumnType = "LONGBLOB"
	Mediumblob schema.ColumnType = "MEDIUMBLOB"
	Tinyblob   schema.ColumnType = "TINYBLOB"
	Json       schema.ColumnType = "JSON"
	Boolean    schema.ColumnType = "BOOLEAN"
	Year       schema.ColumnType = "YEAR"
	Time       schema.ColumnType = "TIME"
)

// columnType maps a MySQL information_schema DATA_TYPE (always lower case, e.g.
// "varchar", "bigint") onto the categorized schema.ColumnType constant. Unknown
// types are upper-cased and passed through so the tool degrades gracefully on
// vendor-specific types rather than failing the whole reflection.
func columnType(dataType string) schema.ColumnType {
	if t, ok := dataTypeMap[strings.ToLower(dataType)]; ok {
		return t
	}
	return schema.ColumnType(strings.ToUpper(dataType))
}

var dataTypeMap = map[string]schema.ColumnType{
	"int":        Int,
	"integer":    Int,
	"varchar":    Varchar,
	"text":       Text,
	"longtext":   Longtext,
	"mediumtext": Mediumtext,
	"tinytext":   Tinytext,
	"date":       Date,
	"datetime":   Datetime,
	"timestamp":  Timestamp,
	"decimal":    Decimal,
	"numeric":    Decimal,
	"float":      Float,
	"double":     Double,
	"enum":       Enum,
	"set":        Set,
	"tinyint":    Tinyint,
	"smallint":   Smallint,
	"mediumint":  Mediumint,
	"bigint":     Bigint,
	"char":       Char,
	"blob":       Blob,
	"longblob":   Longblob,
	"mediumblob": Mediumblob,
	"tinyblob":   Tinyblob,
	"json":       Json,
	"boolean":    Boolean,
	"bool":       Boolean,
	"year":       Year,
	"time":       Time,
}
