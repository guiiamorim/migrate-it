package postgres

import (
	"database/sql"

	_ "github.com/lib/pq"
)

// ConnectionConfig identifies a PostgreSQL database and the namespace within it
// to operate on. SchemaName defaults to "public" when empty.
type ConnectionConfig struct {
	// DSN is a lib/pq connection string or URL, e.g.
	// "postgres://user:pass@host:5432/dbname?sslmode=disable".
	DSN string
	// Database is the logical database name (used to label the reflected schema).
	Database string
	// SchemaName is the PostgreSQL namespace to reflect (default "public").
	SchemaName string
}

// Connection is a PostgreSQL implementation of connection.Connection.
type Connection struct {
	config ConnectionConfig
	db     *sql.DB
}

func NewConnection(config ConnectionConfig) (*Connection, error) {
	if config.SchemaName == "" {
		config.SchemaName = "public"
	}
	return &Connection{config: config}, nil
}

func (c *Connection) Connect() error {
	var err error
	c.db, err = sql.Open("postgres", c.config.DSN)
	if err != nil {
		return err
	}
	return c.db.Ping()
}

func (c *Connection) Disconnect() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *Connection) GetTableNames() ([]string, error) {
	rows, err := c.db.Query(
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		 ORDER BY table_name`, c.config.SchemaName)
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

// DB returns the underlying *sql.DB. Nil until Connect succeeds.
func (c *Connection) DB() *sql.DB { return c.db }

// Database returns the logical database name used to label the schema.
func (c *Connection) Database() string { return c.config.Database }

// SchemaName returns the PostgreSQL namespace being reflected.
func (c *Connection) SchemaName() string { return c.config.SchemaName }
