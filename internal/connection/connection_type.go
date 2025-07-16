package connection

import "fmt"

type Type string

const (
	MySQL    Type = "mysql"
	Postgres Type = "postgres"
	Sqlite   Type = "sqlite"
)

func (t Type) FromString(s string) (Type, error) {
	switch s {
	case "mysql":
		return MySQL, nil
	case "postgres":
		return Postgres, nil
	case "sqlite":
		return Sqlite, nil
	default:
		return "", fmt.Errorf("invalid connection type: %s", s)
	}
}
