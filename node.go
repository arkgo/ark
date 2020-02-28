package ark

type (
	nodeConfig struct {
		Id   int64  `toml:"id"`
		Type string `toml:"type"`
	}
	nodeModule struct {
	}
)

func newNode() *nodeModule {
	return &nodeModule{}
}
