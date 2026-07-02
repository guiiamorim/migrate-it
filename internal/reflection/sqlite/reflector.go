package sqlite

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/guiiamorim/migrateit/internal/schema"
)

// Reflector introspects a SQLite database into a driver-agnostic *schema.Schema
// using the PRAGMA family of statements (SQLite has no information_schema).
type Reflector struct {
	db       *sql.DB
	database string
}

// NewReflector returns a Reflector reading from db. database is used only to
// label the resulting schema (typically the file path).
func NewReflector(db *sql.DB, database string) *Reflector {
	return &Reflector{db: db, database: database}
}

func (r *Reflector) Reflect() (*schema.Schema, error) {
	s := schema.New(r.database)

	names, err := r.tableNames()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		t := &schema.Table{Name: name}
		if err := r.columns(t); err != nil {
			return nil, fmt.Errorf("reflecting columns for %s: %w", name, err)
		}
		if err := r.indexes(t); err != nil {
			return nil, fmt.Errorf("reflecting indexes for %s: %w", name, err)
		}
		if err := r.foreignKeys(t); err != nil {
			return nil, fmt.Errorf("reflecting foreign keys for %s: %w", name, err)
		}
		s.AddTable(t)
	}
	return s, nil
}

func (r *Reflector) tableNames() ([]string, error) {
	rows, err := r.db.Query(
		`SELECT name FROM sqlite_master
		 WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (r *Reflector) columns(t *schema.Table) error {
	rows, err := r.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(t.Name)))
	if err != nil {
		return err
	}
	defer rows.Close()

	type pkCol struct {
		name string
		pos  int
	}
	var pkCols []pkCol
	position := 0
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, declType   string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &declType, &notnull, &dflt, &pk); err != nil {
			return err
		}
		position++
		var defPtr *string
		if dflt.Valid {
			v := dflt.String
			defPtr = &v
		}
		t.Columns = append(t.Columns, &schema.Column{
			Name:       name,
			Type:       columnType(declType),
			Definition: normalizeType(declType),
			Nullable:   notnull == 0,
			Default:    defPtr,
			Position:   position,
		})
		if pk > 0 {
			pkCols = append(pkCols, pkCol{name: name, pos: pk})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(pkCols) > 0 {
		sort.Slice(pkCols, func(i, j int) bool { return pkCols[i].pos < pkCols[j].pos })
		cols := make([]string, len(pkCols))
		for i, c := range pkCols {
			cols[i] = c.name
		}
		t.PrimaryKey = &schema.Constraint{Name: t.Name + "_pk", Type: PrimaryKey, Columns: cols}
	}
	return nil
}

// indexes reflects unique and plain indexes. Indexes that SQLite auto-creates to
// back a primary key (origin "pk") are skipped, since the PK is captured from
// table_info.
func (r *Reflector) indexes(t *schema.Table) error {
	rows, err := r.db.Query(fmt.Sprintf("PRAGMA index_list(%s)", quoteIdent(t.Name)))
	if err != nil {
		return err
	}
	defer rows.Close()

	type idxMeta struct {
		name   string
		unique bool
	}
	var metas []idxMeta
	for rows.Next() {
		var (
			seq, unique, partial int
			name, origin         string
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return err
		}
		if origin == "pk" {
			continue
		}
		metas = append(metas, idxMeta{name: name, unique: unique == 1})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range metas {
		cols, err := r.indexColumns(m.name)
		if err != nil {
			return err
		}
		c := &schema.Constraint{Name: m.name, Columns: cols, Type: Index}
		if m.unique {
			c.Type = Unique
		}
		t.Constraints = append(t.Constraints, c)
	}
	return nil
}

func (r *Reflector) indexColumns(index string) ([]string, error) {
	rows, err := r.db.Query(fmt.Sprintf("PRAGMA index_info(%s)", quoteIdent(index)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		cols = append(cols, name.String)
	}
	return cols, rows.Err()
}

// foreignKeys reflects foreign keys via PRAGMA foreign_key_list. SQLite foreign
// keys are anonymous, so a stable name is synthesised from the table and the
// referencing columns (not the volatile PRAGMA id).
func (r *Reflector) foreignKeys(t *schema.Table) error {
	rows, err := r.db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(t.Name)))
	if err != nil {
		return err
	}
	defer rows.Close()

	collected := map[int]*schema.ForeignKey{}
	var order []int
	for rows.Next() {
		var (
			id, seq                         int
			refTable, from, to              string
			onUpdate, onDelete, matchClause string
		)
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &matchClause); err != nil {
			return err
		}
		fk := collected[id]
		if fk == nil {
			fk = &schema.ForeignKey{
				RefTable: refTable,
				OnUpdate: normalizeAction(onUpdate),
				OnDelete: normalizeAction(onDelete),
			}
			collected[id] = fk
			order = append(order, id)
		}
		fk.Columns = append(fk.Columns, from)
		fk.RefColumns = append(fk.RefColumns, to)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	sort.Ints(order)
	for _, id := range order {
		fk := collected[id]
		// SQLite foreign keys are anonymous, and the PRAGMA id is assigned in a
		// declaration-order-dependent way. Derive a stable, content-based name so
		// that two logically identical schemas reflect to identical FK names.
		fk.Name = fmt.Sprintf("fk_%s_%s", t.Name, strings.Join(fk.Columns, "_"))
		t.ForeignKeys = append(t.ForeignKeys, fk)
	}
	return nil
}

// normalizeType trims and standardises a declared type for stable diffing.
func normalizeType(declared string) string {
	d := strings.TrimSpace(declared)
	if d == "" {
		return "BLOB"
	}
	return d
}

// normalizeAction drops SQLite's "NO ACTION" default so it does not show up as a
// spurious difference and so generated DDL stays terse.
func normalizeAction(action string) string {
	if strings.EqualFold(action, "NO ACTION") {
		return ""
	}
	return action
}

func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}
