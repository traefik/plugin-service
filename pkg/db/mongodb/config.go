package mongodb

// Config holds the MongoDB configuration.
type Config struct {
	URI      string
	Database string
	MinPool  uint64
	MaxPool  uint64
}
