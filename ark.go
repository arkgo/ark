package ark

import (
	. "github.com/arkgo/base"
)

type (
	arkCore struct {
		Config *arkConfig

		Node    *nodeModule
		Serial  *serialModule
		Basic   *basicModule
		Logger  *loggerModule
		Bus     *busModule
		Store   *storeModule
		Session *sessionModule
		Cache   *cacheModule
	}
	arkConfig struct {
		Name string `toml:"name"`
		Mode string `toml:"mode"`

		Node nodeConfig `toml:"node"`

		Basic basicConfig           `toml:"basic"`
		Lang  map[string]langConfig `toml:"lang"`

		Serial serialConfig `toml:"serial"`

		Logger LoggerConfig         `toml:"logger"`
		Bus    map[string]BusConfig `toml:"bus"`

		File  FileConfig             `toml:"file"`
		Store map[string]StoreConfig `toml:"store"`

		Session map[string]SessionConfig `toml:"session"`
		Cache   map[string]CacheConfig   `toml:"cache"`

		Setting Map `toml:"setting"`
	}
)

// Run 是启动方法
func Run() {
	ark.Logger.Debug("ark running")
}

func Driver(name string, driver Any) {
	switch drv := driver.(type) {
	case LoggerDriver:
		ark.Logger.Driver(name, drv)
	case BusDriver:
		ark.Bus.Driver(name, drv)
	case StoreDriver:
		ark.Store.Driver(name, drv)
	case SessionDriver:
		ark.Session.Driver(name, drv)
	}
}
