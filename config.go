package ark

type (
	configConfig struct {
		msg string
	}

	// LoggerConfig 是日志配置类
	LoggerConfig struct {
		Driver  string `toml:"driver"`
		Flag    string `toml:"flag"`
		Console bool   `toml:"console"`
		Level   string `toml:"level"`
		Format  string `toml:"format"`
		Setting Map    `toml:"setting"`
	}
)
