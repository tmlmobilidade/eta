package types

// ClickhouseClientParams contains the configuration for connecting to ClickHouse.
type ClickhouseClientParams struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}
