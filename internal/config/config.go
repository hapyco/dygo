package config

import "fmt"

const (
	defaultServerHost = "127.0.0.1"
	defaultServerPort = 6790
)

// Config contains dygo runtime settings.
type Config struct {
	Server Server
}

// Server contains HTTP server settings.
type Server struct {
	Host string
	Port int
}

// Default returns the built-in dygo configuration.
func Default() Config {
	return Config{
		Server: Server{
			Host: defaultServerHost,
			Port: defaultServerPort,
		},
	}
}

// Address returns the host:port pair used by the server.
func (s Server) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
