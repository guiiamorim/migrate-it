package sqlite

import (
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// SQLite uses dynamic type affinity, but columns still carry a declared type.
// These constants name the affinities the declared type resolves to.
const (
	Integer schema.ColumnType = "INTEGER"
	Real    schema.ColumnType = "REAL"
	Text    schema.ColumnType = "TEXT"
	Blob    schema.ColumnType = "BLOB"
	Numeric schema.ColumnType = "NUMERIC"
)

// columnType derives the SQLite type affinity from a declared type, following
// the rules in https://www.sqlite.org/datatype3.html#determination_of_column_affinity.
func columnType(declared string) schema.ColumnType {
	d := strings.ToUpper(declared)
	switch {
	case strings.Contains(d, "INT"):
		return Integer
	case strings.Contains(d, "CHAR"), strings.Contains(d, "CLOB"), strings.Contains(d, "TEXT"):
		return Text
	case d == "" || strings.Contains(d, "BLOB"):
		return Blob
	case strings.Contains(d, "REAL"), strings.Contains(d, "FLOA"), strings.Contains(d, "DOUB"):
		return Real
	default:
		return Numeric
	}
}
