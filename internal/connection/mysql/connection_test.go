package mysql

import (
	"os"
	"strconv"
	"strings"
	"testing"

	driver "github.com/go-sql-driver/mysql"
)

// testConfig builds a connection config from the MIGRATEIT_MYSQL_DSN environment
// variable (standard go-sql-driver DSN, e.g. "root:pass@tcp(localhost:3306)/db").
// These are integration tests: they are skipped unless a DSN is provided, so a
// checkout without a running MySQL server (such as CI) stays green.
func testConfig(t *testing.T) ConnectionConfig {
	t.Helper()
	dsn := os.Getenv("MIGRATEIT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MIGRATEIT_MYSQL_DSN to run MySQL connection integration tests")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("invalid MIGRATEIT_MYSQL_DSN: %s", err)
	}
	host, port := cfg.Addr, 3306
	if h, p, ok := strings.Cut(cfg.Addr, ":"); ok {
		host = h
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return ConnectionConfig{
		Host:     host,
		Port:     port,
		Username: cfg.User,
		Password: cfg.Passwd,
		Database: cfg.DBName,
	}
}

func TestCanConnectAndDisconnectToDatabase(t *testing.T) {
	conn, err := NewConnection(testConfig(t))
	if err != nil {
		t.Fatalf("Error creating connection: %s", err)
	}

	if err := conn.Connect(); err != nil {
		t.Fatalf("Error connecting to database: %s", err)
	}

	if err := conn.Disconnect(); err != nil {
		t.Fatalf("Error disconnecting from database: %s", err)
	}
}

func TestCanGetTableNames(t *testing.T) {
	conn, err := NewConnection(testConfig(t))
	if err != nil {
		t.Fatalf("Error creating connection: %s", err)
	}

	if err := conn.Connect(); err != nil {
		t.Fatalf("Error connecting to database: %s", err)
	}
	defer conn.Disconnect()

	tables, err := conn.GetTableNames()
	if err != nil {
		t.Fatalf("Error getting table names: %s", err)
	}

	for _, tbl := range tables {
		t.Log(tbl)
	}
}
