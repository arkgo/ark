package ark

type (
	nodeConfig struct {
		Id   int64  `toml:"id"`
		Type string `toml:"type"`
	}
	nodeModule struct {
		config nodeConfig
	}
)

func newNode(config nodeConfig) *nodeModule {
	if config.Id <= 0 {
		config.Id = 1
	}
	if config.Type == "" {
		config.Type = GATEWAY
	}
	return &nodeModule{
		config: config,
	}
}
