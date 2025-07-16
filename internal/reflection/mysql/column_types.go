package mysql

import (
	"github.com/guiiamorim/migrateit/internal/reflection"
)

const (
	Int        reflection.ColumnType = "INT"
	Varchar    reflection.ColumnType = "VARCHAR"
	Text       reflection.ColumnType = "TEXT"
	Longtext   reflection.ColumnType = "LONGTEXT"
	Mediumtext reflection.ColumnType = "MEDIUMTEXT"
	Tinytext   reflection.ColumnType = "TINYTEXT"
	Date       reflection.ColumnType = "DATE"
	Datetime   reflection.ColumnType = "DATETIME"
	Timestamp  reflection.ColumnType = "TIMESTAMP"
	Decimal    reflection.ColumnType = "DECIMAL"
	Float      reflection.ColumnType = "FLOAT"
	Double     reflection.ColumnType = "DOUBLE"
	Enum       reflection.ColumnType = "ENUM"
	Set        reflection.ColumnType = "SET"
	Tinyint    reflection.ColumnType = "TINYINT"
	Smallint   reflection.ColumnType = "SMALLINT"
	Mediumint  reflection.ColumnType = "MEDIUMINT"
	Bigint     reflection.ColumnType = "BIGINT"
	Char       reflection.ColumnType = "CHAR"
	Blob       reflection.ColumnType = "BLOB"
	Longblob   reflection.ColumnType = "LONGBLOB"
	Mediumblob reflection.ColumnType = "MEDIUMBLOB"
	Tinyblob   reflection.ColumnType = "TINYBLOB"
	Json       reflection.ColumnType = "JSON"
	Boolean    reflection.ColumnType = "BOOLEAN"
	Year       reflection.ColumnType = "YEAR"
	Time       reflection.ColumnType = "TIME"
)

func numericTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Int,
		Tinyint,
		Smallint,
		Mediumint,
		Bigint,
		Decimal,
		Float,
		Double,
	}
}

func stringTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Varchar,
		Text,
		Longtext,
		Mediumtext,
		Tinytext,
		Char,
		Json,
	}
}

func dateTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Date,
		Datetime,
		Timestamp,
		Year,
		Time,
	}
}

func booleanTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Boolean,
		Tinyint,
	}
}

func jsonTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Json,
		Longtext,
	}
}

func blobTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Blob,
		Longblob,
		Mediumblob,
		Tinyblob,
	}
}

func enumTypes() []reflection.ColumnType {
	return []reflection.ColumnType{
		Enum,
		Set,
	}
}
