package mysql

import "testing"

var config = ConnectionConfig{
	Host:     "localhost",
	Port:     3307,
	Username: "root",
	Password: "gestao_empresarial@@2021",
	Database: "cdcaixa-admin",
}

func TestCanConnectAndDisconnectToDatabase(t *testing.T) {
	conn, err := NewConnection(config)
	if err != nil {
		t.Fatalf("Error creating connection: %s", err)
	}

	err = conn.Connect()
	if err != nil {
		t.Fatalf("Error connecting to database: %s", err)
	}

	err = conn.Disconnect()
	if err != nil {
		t.Fatalf("Error disconnecting from database: %s", err)
	}
}

func TestCanGetTableNames(t *testing.T) {
	conn, err := NewConnection(config)
	if err != nil {
		t.Fatalf("Error creating connection: %s", err)
	}

	err = conn.Connect()
	if err != nil {
		t.Fatalf("Error connecting to database: %s", err)
	}

	tables, err := conn.GetTableNames()
	if err != nil {
		t.Fatalf("Error getting table names: %s", err)
	}

	if len(tables) == 0 {
		t.Fatalf("No tables found")
	}

	for _, tbl := range tables {
		t.Log(tbl) // Print each table name to the console
	}

	err = conn.Disconnect()
	if err != nil {
		t.Fatalf("Error disconnecting from database: %s", err)
	}
}
