package postgres

import (
	"database/sql"
	"fmt"

	"github.com/guiiamorim/migrateit/internal/schema"
	"github.com/lib/pq"
)

// Reflector introspects a PostgreSQL schema (namespace) into a
// driver-agnostic *schema.Schema.
type Reflector struct {
	db         *sql.DB
	database   string
	schemaName string
}

// NewReflector returns a Reflector reading namespace schemaName from db.
// database is used only to label the resulting schema; schemaName defaults to
// "public" when empty.
func NewReflector(db *sql.DB, database, schemaName string) *Reflector {
	if schemaName == "" {
		schemaName = "public"
	}
	return &Reflector{db: db, database: database, schemaName: schemaName}
}

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
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		ORDER BY table_name`
	rows, err := r.db.Query(q, r.schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []*schema.Table
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, &schema.Table{Name: name})
	}
	return tables, rows.Err()
}

func (r *Reflector) columns(t *schema.Table) error {
	const q = `
		SELECT column_name, data_type, udt_name,
		       character_maximum_length, numeric_precision, numeric_scale,
		       is_nullable, column_default, ordinal_position,
		       is_identity, COALESCE(identity_generation, '')
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`
	rows, err := r.db.Query(q, r.schemaName, t.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			name, dataType, udt, nullable, isIdentity, identityGen string
			charLen, numPrec, numScale                             sql.NullInt64
			def                                                    sql.NullString
			position                                               int
		)
		if err := rows.Scan(&name, &dataType, &udt, &charLen, &numPrec, &numScale,
			&nullable, &def, &position, &isIdentity, &identityGen); err != nil {
			return err
		}

		extra := ""
		var defPtr *string
		if isIdentity == "YES" {
			extra = "GENERATED " + identityGen + " AS IDENTITY"
		} else if def.Valid {
			v := def.String
			defPtr = &v
		}

		t.Columns = append(t.Columns, &schema.Column{
			Name:       name,
			Type:       columnType(dataType),
			Definition: buildDefinition(dataType, udt, charLen, numPrec, numScale),
			Nullable:   nullable == "YES",
			Default:    defPtr,
			Extra:      extra,
			Position:   position,
		})
	}
	return rows.Err()
}

// buildDefinition reconstructs a valid PostgreSQL type string from the parts
// reported by information_schema (which, unlike MySQL, has no single column for
// the full type).
func buildDefinition(dataType, udt string, charLen, numPrec, numScale sql.NullInt64) string {
	switch dataType {
	case "character varying", "character", "bit", "bit varying":
		if charLen.Valid {
			return fmt.Sprintf("%s(%d)", dataType, charLen.Int64)
		}
	case "numeric", "decimal":
		if numPrec.Valid {
			if numScale.Valid && numScale.Int64 != 0 {
				return fmt.Sprintf("numeric(%d,%d)", numPrec.Int64, numScale.Int64)
			}
			return fmt.Sprintf("numeric(%d)", numPrec.Int64)
		}
	case "ARRAY":
		// udt is like "_int4"; render as element type with [] suffix.
		return arrayDefinition(udt)
	case "USER-DEFINED":
		return udt
	}
	return dataType
}

func arrayDefinition(udt string) string {
	elem := udt
	if len(udt) > 0 && udt[0] == '_' {
		elem = udt[1:]
	}
	return elem + "[]"
}

// indexes reflects primary keys, unique keys and plain indexes via pg_catalog,
// which preserves multi-column ordering reliably.
func (r *Reflector) indexes(t *schema.Table) error {
	const q = `
		SELECT i.relname AS index_name, ix.indisunique, ix.indisprimary, a.attname, k.ord
		FROM pg_class tbl
		JOIN pg_namespace n ON n.oid = tbl.relnamespace
		JOIN pg_index ix ON tbl.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
		JOIN pg_attribute a ON a.attrelid = tbl.oid AND a.attnum = k.attnum
		WHERE tbl.relname = $1 AND n.nspname = $2 AND tbl.relkind = 'r'
		ORDER BY index_name, k.ord`
	rows, err := r.db.Query(q, t.Name, r.schemaName)
	if err != nil {
		return err
	}
	defer rows.Close()

	type idx struct {
		unique, primary bool
		columns         []string
	}
	collected := map[string]*idx{}
	var order []string
	for rows.Next() {
		var indexName, column string
		var unique, primary bool
		var ord int
		if err := rows.Scan(&indexName, &unique, &primary, &column, &ord); err != nil {
			return err
		}
		if collected[indexName] == nil {
			collected[indexName] = &idx{unique: unique, primary: primary}
			order = append(order, indexName)
		}
		collected[indexName].columns = append(collected[indexName].columns, column)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range order {
		i := collected[name]
		c := &schema.Constraint{Name: name, Columns: i.columns}
		switch {
		case i.primary:
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

// foreignKeys reflects foreign keys from pg_constraint, using the conkey/confkey
// attribute arrays so composite keys keep their column ordering.
func (r *Reflector) foreignKeys(t *schema.Table) error {
	const q = `
		SELECT c.conname,
		       (SELECT array_agg(att.attname ORDER BY k.ord)
		          FROM unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
		          JOIN pg_attribute att ON att.attrelid = c.conrelid AND att.attnum = k.attnum) AS columns,
		       cl.relname AS ref_table,
		       (SELECT array_agg(att.attname ORDER BY k.ord)
		          FROM unnest(c.confkey) WITH ORDINALITY AS k(attnum, ord)
		          JOIN pg_attribute att ON att.attrelid = c.confrelid AND att.attnum = k.attnum) AS ref_columns,
		       c.confdeltype, c.confupdtype
		FROM pg_constraint c
		JOIN pg_class tbl ON tbl.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = tbl.relnamespace
		JOIN pg_class cl ON cl.oid = c.confrelid
		WHERE c.contype = 'f' AND tbl.relname = $1 AND n.nspname = $2
		ORDER BY c.conname`
	rows, err := r.db.Query(q, t.Name, r.schemaName)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, refTable, delType, updType string
		var cols, refCols pq.StringArray
		if err := rows.Scan(&name, &cols, &refTable, &refCols, &delType, &updType); err != nil {
			return err
		}
		t.ForeignKeys = append(t.ForeignKeys, &schema.ForeignKey{
			Name:       name,
			Columns:    cols,
			RefTable:   refTable,
			RefColumns: refCols,
			OnDelete:   fkAction(delType),
			OnUpdate:   fkAction(updType),
		})
	}
	return rows.Err()
}
