package sqlite

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// ConnectionConfig identifies a SQLite database file.
type ConnectionConfig struct {
	// Path is the database file path (or ":memory:").
	Path string
}

// Connection is a SQLite implementation of connection.Connection.
type Connection struct {
	config ConnectionConfig
	db     *sql.DB
}

func NewConnection(config ConnectionConfig) (*Connection, error) {
	return &Connection{config: config}, nil
}

func (c *Connection) Connect() error {
	var err error
	c.db, err = sql.Open("sqlite", c.config.Path)
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

// DB returns the underlying *sql.DB. Nil until Connect succeeds.
func (c *Connection) DB() *sql.DB { return c.db }

// Database returns the file path, used to label the reflected schema.
func (c *Connection) Database() string { return c.config.Path }
