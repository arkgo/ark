package ark

type (
	nodeConfig struct {
		Id   int64  `toml:"id"`
		Type string `toml:"type"`
		Temp string `toml:"temp"`
	}
	nodeModule struct {
	}
)

func newNode() *nodeModule {
	return &nodeModule{}
}
