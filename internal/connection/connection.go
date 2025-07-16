package connection

type Connection interface {
	Connect() error
	Disconnect() error
	GetTableNames() ([]string, error)
}
