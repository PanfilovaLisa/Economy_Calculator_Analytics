package configs

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Db     DbConfig
	Server ServerConfig
	Data   DataConfig
	Output OutputConfig
}

type DbConfig struct {
	Dsn string
}

type ServerConfig struct {
	Addr           string
	MaxUploadBytes int64
	Workers        int
}

type DataConfig struct {
	IndicesDir string
}

type OutputConfig struct {
	Dir          string
	DecimalComma bool
}

func LoadConfig() *Config {
	_ = godotenv.Load(".env")

	cfg := &Config{
		Db: DbConfig{
			Dsn: os.Getenv("DSN"),
		},
		Server: ServerConfig{
			Addr:           getenv("HTTP_ADDR", ":8082"),
			MaxUploadBytes: 32 << 20,
			Workers:        4,
		},
		Data: DataConfig{
			IndicesDir: getenv("INDICES_DIR", "./data"),
		},
		Output: OutputConfig{
			Dir:          getenv("OUTPUT_DIR", "../history"),
			DecimalComma: true,
		},
	}
	if cfg.Db.Dsn == "" {
		return nil
	}
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
