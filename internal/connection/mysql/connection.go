package mysql

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
)

type ConnectionConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

type Connection struct {
	config ConnectionConfig
	db     *sql.DB
}

func NewConnection(config ConnectionConfig) (*Connection, error) {
	return &Connection{
		config: config,
	}, nil
}

func (c *Connection) Connect() error {
	confg := mysql.Config{
		User:   c.config.Username,
		Passwd: c.config.Password,
		Net:    "tcp",
		Addr:   fmt.Sprintf("%s:%d", c.config.Host, c.config.Port),
		DBName: c.config.Database,
	}

	var err error
	c.db, err = sql.Open("mysql", confg.FormatDSN())
	if err != nil {
		return err
	}

	err = c.db.Ping()
	return err
}

func (c *Connection) Disconnect() error {
	if c.db != nil {
		return c.db.Close()
	}

	return nil
}

// DB returns the underlying *sql.DB so that the reflection layer can query
// information_schema directly. It is nil until Connect has succeeded.
func (c *Connection) DB() *sql.DB {
	return c.db
}

// Database returns the name of the schema this connection targets.
func (c *Connection) Database() string {
	return c.config.Database
}

func (c *Connection) GetTableNames() ([]string, error) {
	rows, err := c.db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}

	var tableNames []string
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			return nil, err
		}

		tableNames = append(tableNames, tableName)
	}

	return tableNames, nil
}
