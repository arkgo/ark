package ark

type (
	configConfig struct {
		Name string `toml:"name"`
		Mode string `toml:"mode"`

		Logger LoggerConfig         `toml:"logger"`
		Bus    map[string]BusConfig `toml:"bus"`
	}
)
