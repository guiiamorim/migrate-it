// Command migrate-it reflects two databases, diffs their schemas and prints a
// migration script whose statement order respects foreign-key dependencies.
//
// Usage:
//
//	migrate-it -source <dsn> -target <dsn> [-format sql|json|yaml] [-out file]
//
// The driver is selected by the DSN scheme:
//
//	mysql://user:pass@tcp(host:3306)/dbname
//	postgres://user:pass@host:5432/dbname?sslmode=disable
//	sqlite://path/to/file.db          (or sqlite:path, or a bare *.db / *.sqlite path)
//
// The migration transforms the SOURCE schema into the TARGET schema; the
// target's driver determines the dialect of the generated SQL. -format selects
// the representation (sql migration script, the diff as json/yaml, or a
// git-diff-style html report) and -out writes it to a file instead of stdout.
package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	mysqlconn "github.com/guiiamorim/migrateit/internal/connection/mysql"
	pgconn "github.com/guiiamorim/migrateit/internal/connection/postgres"
	sqliteconn "github.com/guiiamorim/migrateit/internal/connection/sqlite"
	"github.com/guiiamorim/migrateit/internal/output"
	"github.com/guiiamorim/migrateit/internal/reflection"
	mysqlref "github.com/guiiamorim/migrateit/internal/reflection/mysql"
	pgref "github.com/guiiamorim/migrateit/internal/reflection/postgres"
	sqliteref "github.com/guiiamorim/migrateit/internal/reflection/sqlite"
	"github.com/guiiamorim/migrateit/internal/schema"

	"github.com/go-sql-driver/mysql"
)

func main() {
	source := flag.String("source", "", "source DSN (the schema to migrate FROM)")
	target := flag.String("target", "", "target DSN (the schema to migrate TO)")
	format := flag.String("format", output.Default, "output format: sql, json, yaml, html")
	out := flag.String("out", "", "write output to this file instead of stdout")
	flag.Parse()

	if *source == "" || *target == "" {
		fmt.Fprintln(os.Stderr, "both -source and -target DSNs are required")
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*source, *target, *format, *out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(sourceDSN, targetDSN, format, outPath string) error {
	// Fail fast on a bad format before doing any database work.
	if err := output.Validate(format); err != nil {
		return err
	}

	srcSchema, _, err := open(sourceDSN)
	if err != nil {
		return fmt.Errorf("reflecting source: %w", err)
	}
	tgtSchema, dialect, err := open(targetDSN)
	if err != nil {
		return fmt.Errorf("reflecting target: %w", err)
	}

	diff := schema.Compare(srcSchema, tgtSchema)

	rendered, err := output.Render(diff, format, dialect)
	if err != nil {
		return err
	}
	return writeOutput(outPath, rendered)
}

// writeOutput sends rendered bytes to a file, or to stdout when path is empty.
func writeOutput(path string, data []byte) error {
	if path == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", path, len(data))
	return nil
}

// open connects to the database named by a scheme-prefixed DSN, reflects it, and
// returns its schema plus the dialect for its driver.
func open(dsn string) (*schema.Schema, schema.Dialect, error) {
	scheme, rest := splitScheme(dsn)
	switch scheme {
	case "mysql":
		return openMySQL(rest)
	case "postgres", "postgresql":
		return openPostgres(dsn)
	case "sqlite", "sqlite3", "file":
		return openSQLite(rest)
	default:
		return nil, nil, fmt.Errorf("unsupported or missing DSN scheme %q (use mysql://, postgres:// or sqlite://)", scheme)
	}
}

func openMySQL(dsn string) (*schema.Schema, schema.Dialect, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing MySQL DSN: %w", err)
	}
	host, port := splitHostPort(cfg.Addr, 3306)
	conn, err := mysqlconn.NewConnection(mysqlconn.ConnectionConfig{
		Host: host, Port: port, Username: cfg.User, Password: cfg.Passwd, Database: cfg.DBName,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := conn.Connect(); err != nil {
		return nil, nil, err
	}
	defer conn.Disconnect()

	var r reflection.Reflector = mysqlref.NewReflector(conn.DB(), conn.Database())
	s, err := r.Reflect()
	return s, mysqlref.NewDialect(), err
}

func openPostgres(dsn string) (*schema.Schema, schema.Dialect, error) {
	database, schemaName := postgresIdentity(dsn)
	conn, err := pgconn.NewConnection(pgconn.ConnectionConfig{
		DSN: dsn, Database: database, SchemaName: schemaName,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := conn.Connect(); err != nil {
		return nil, nil, err
	}
	defer conn.Disconnect()

	var r reflection.Reflector = pgref.NewReflector(conn.DB(), conn.Database(), conn.SchemaName())
	s, err := r.Reflect()
	return s, pgref.NewDialect(), err
}

func openSQLite(path string) (*schema.Schema, schema.Dialect, error) {
	conn, err := sqliteconn.NewConnection(sqliteconn.ConnectionConfig{Path: path})
	if err != nil {
		return nil, nil, err
	}
	if err := conn.Connect(); err != nil {
		return nil, nil, err
	}
	defer conn.Disconnect()

	var r reflection.Reflector = sqliteref.NewReflector(conn.DB(), conn.Database())
	s, err := r.Reflect()
	return s, sqliteref.NewDialect(), err
}

// splitScheme separates a "scheme://rest" DSN. A bare path ending in a SQLite
// file extension is treated as sqlite for convenience.
func splitScheme(dsn string) (scheme, rest string) {
	if s, r, ok := strings.Cut(dsn, "://"); ok {
		return s, r
	}
	if s, r, ok := strings.Cut(dsn, ":"); ok {
		// "sqlite:path" form (no //).
		if s == "sqlite" || s == "sqlite3" || s == "file" {
			return s, r
		}
	}
	if strings.HasSuffix(dsn, ".db") || strings.HasSuffix(dsn, ".sqlite") || dsn == ":memory:" {
		return "sqlite", dsn
	}
	return "", dsn
}

// postgresIdentity extracts the database name and target namespace from a
// postgres URL. SchemaName defaults to "public" unless a search_path query
// parameter is present.
func postgresIdentity(dsn string) (database, schemaName string) {
	schemaName = "public"
	u, err := url.Parse(dsn)
	if err != nil {
		return "", schemaName
	}
	database = strings.TrimPrefix(u.Path, "/")
	if sp := u.Query().Get("search_path"); sp != "" {
		schemaName = sp
	}
	return database, schemaName
}

func splitHostPort(addr string, defaultPort int) (string, int) {
	host, port := addr, defaultPort
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
		if p, err := strconv.Atoi(addr[i+1:]); err == nil {
			port = p
		}
	}
	if host == "" {
		host = "localhost"
	}
	return host, port
}
