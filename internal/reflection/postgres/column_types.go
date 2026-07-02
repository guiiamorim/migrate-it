package postgres

import (
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

const (
	Smallint    schema.ColumnType = "SMALLINT"
	Integer     schema.ColumnType = "INTEGER"
	Bigint      schema.ColumnType = "BIGINT"
	Numeric     schema.ColumnType = "NUMERIC"
	Real        schema.ColumnType = "REAL"
	Double      schema.ColumnType = "DOUBLE PRECISION"
	Varchar     schema.ColumnType = "VARCHAR"
	Char        schema.ColumnType = "CHAR"
	Text        schema.ColumnType = "TEXT"
	Boolean     schema.ColumnType = "BOOLEAN"
	Date        schema.ColumnType = "DATE"
	Timestamp   schema.ColumnType = "TIMESTAMP"
	TimestampTZ schema.ColumnType = "TIMESTAMPTZ"
	Time        schema.ColumnType = "TIME"
	JSON        schema.ColumnType = "JSON"
	JSONB       schema.ColumnType = "JSONB"
	UUID        schema.ColumnType = "UUID"
	Bytea       schema.ColumnType = "BYTEA"
	Serial      schema.ColumnType = "SERIAL"
)

// columnType maps a PostgreSQL information_schema data_type onto a categorized
// schema.ColumnType. Unknown types are upper-cased and passed through so the
// tool degrades gracefully rather than failing reflection.
func columnType(dataType string) schema.ColumnType {
	if t, ok := dataTypeMap[strings.ToLower(dataType)]; ok {
		return t
	}
	return schema.ColumnType(strings.ToUpper(dataType))
}

var dataTypeMap = map[string]schema.ColumnType{
	"smallint":                    Smallint,
	"integer":                     Integer,
	"bigint":                      Bigint,
	"numeric":                     Numeric,
	"decimal":                     Numeric,
	"real":                        Real,
	"double precision":            Double,
	"character varying":           Varchar,
	"varchar":                     Varchar,
	"character":                   Char,
	"char":                        Char,
	"text":                        Text,
	"boolean":                     Boolean,
	"date":                        Date,
	"timestamp without time zone": Timestamp,
	"timestamp with time zone":    TimestampTZ,
	"time without time zone":      Time,
	"time with time zone":         Time,
	"json":                        JSON,
	"jsonb":                       JSONB,
	"uuid":                        UUID,
	"bytea":                       Bytea,
}
