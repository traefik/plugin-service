package db

// Config holds FaunaDB configuration.
type Config struct {
	Database string
	Endpoint string
	Secret   string
}
