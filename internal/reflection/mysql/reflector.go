package mysql

import (
	"database/sql"
	"fmt"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// Reflector introspects a MySQL database via information_schema and builds a
// driver-agnostic *schema.Schema.
type Reflector struct {
	db       *sql.DB
	database string
}

// NewReflector returns a Reflector reading from db for the given database name.
func NewReflector(db *sql.DB, database string) *Reflector {
	return &Reflector{db: db, database: database}
}

// Reflect reads every table in the configured database, including columns,
// primary keys, secondary indexes/constraints and foreign keys.
func (r *Reflector) Reflect() (*schema.Schema, error) {
	s := schema.New(r.database)

	tables, err := r.tables()
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		if err := r.columns(t); err != nil {
			return nil, fmt.Errorf("reflecting columns for %s: %w", t.Name, err)
		}
		if err := r.indexes(t); err != nil {
			return nil, fmt.Errorf("reflecting indexes for %s: %w", t.Name, err)
		}
		if err := r.foreignKeys(t); err != nil {
			return nil, fmt.Errorf("reflecting foreign keys for %s: %w", t.Name, err)
		}
		s.AddTable(t)
	}
	return s, nil
}

func (r *Reflector) tables() ([]*schema.Table, error) {
	const q = `
		SELECT TABLE_NAME, ENGINE, TABLE_COLLATION
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`
	rows, err := r.db.Query(q, r.database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []*schema.Table
	for rows.Next() {
		var name string
		var engine, collation sql.NullString
		if err := rows.Scan(&name, &engine, &collation); err != nil {
			return nil, err
		}
		tables = append(tables, &schema.Table{
			Name:      name,
			Engine:    engine.String,
			Collation: collation.String,
			Charset:   charsetFromCollation(collation.String),
		})
	}
	return tables, rows.Err()
}

func (r *Reflector) columns(t *schema.Table) error {
	const q = `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE,
		       COLUMN_DEFAULT, EXTRA, ORDINAL_POSITION,
		       CHARACTER_SET_NAME, COLLATION_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`
	rows, err := r.db.Query(q, r.database, t.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			name, dataType, colType, nullable string
			def, extra, charset, collation    sql.NullString
			position                          int
		)
		if err := rows.Scan(&name, &dataType, &colType, &nullable, &def, &extra,
			&position, &charset, &collation); err != nil {
			return err
		}
		var defPtr *string
		if def.Valid {
			v := def.String
			defPtr = &v
		}
		t.Columns = append(t.Columns, &schema.Column{
			Name:       name,
			Type:       columnType(dataType),
			Definition: colType,
			Nullable:   nullable == "YES",
			Default:    defPtr,
			Extra:      extra.String,
			Position:   position,
			Charset:    charset.String,
			Collation:  collation.String,
		})
	}
	return rows.Err()
}

// indexes reads PRIMARY/UNIQUE/plain indexes from STATISTICS. Multi-column
// indexes are reassembled in SEQ_IN_INDEX order.
func (r *Reflector) indexes(t *schema.Table) error {
	const q = `
		SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME, SEQ_IN_INDEX
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`
	rows, err := r.db.Query(q, r.database, t.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	type idx struct {
		unique  bool
		columns []string
	}
	collected := map[string]*idx{}
	var order []string
	for rows.Next() {
		var indexName, columnName string
		var nonUnique, seq int
		if err := rows.Scan(&indexName, &nonUnique, &columnName, &seq); err != nil {
			return err
		}
		if collected[indexName] == nil {
			collected[indexName] = &idx{unique: nonUnique == 0}
			order = append(order, indexName)
		}
		collected[indexName].columns = append(collected[indexName].columns, columnName)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range order {
		i := collected[name]
		c := &schema.Constraint{Name: name, Columns: i.columns}
		switch {
		case name == "PRIMARY":
			c.Type = PrimaryKey
			t.PrimaryKey = c
			continue
		case i.unique:
			c.Type = Unique
		default:
			c.Type = Index
		}
		t.Constraints = append(t.Constraints, c)
	}
	return nil
}

func (r *Reflector) foreignKeys(t *schema.Table) error {
	const q = `
		SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME,
		       k.REFERENCED_COLUMN_NAME, r.DELETE_RULE, r.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE k
		JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		  ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA
		 AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME
		WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ?
		  AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`
	rows, err := r.db.Query(q, r.database, t.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	collected := map[string]*schema.ForeignKey{}
	var order []string
	for rows.Next() {
		var name, column, refTable, refColumn, onDelete, onUpdate string
		if err := rows.Scan(&name, &column, &refTable, &refColumn, &onDelete, &onUpdate); err != nil {
			return err
		}
		fk := collected[name]
		if fk == nil {
			fk = &schema.ForeignKey{Name: name, RefTable: refTable, OnDelete: onDelete, OnUpdate: onUpdate}
			collected[name] = fk
			order = append(order, name)
		}
		fk.Columns = append(fk.Columns, column)
		fk.RefColumns = append(fk.RefColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range order {
		t.ForeignKeys = append(t.ForeignKeys, collected[name])
	}
	return nil
}

// charsetFromCollation derives the charset name from a collation like
// "utf8mb4_0900_ai_ci" -> "utf8mb4". It is a best-effort convenience only.
func charsetFromCollation(collation string) string {
	for i := 0; i < len(collation); i++ {
		if collation[i] == '_' {
			return collation[:i]
		}
	}
	return collation
}
