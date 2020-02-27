package ark

import (
	"time"

	"github.com/arkgo/asset/hashring"
	. "github.com/arkgo/base"
)

type (
	StoreConfig struct {
		Driver  string `toml:"driver"`
		Weight  int    `toml:"weight"`
		Cache   string `toml:"cache"`
		Browse  bool   `toml:"browse"`
		Preview bool   `toml:"preview"`
		Setting Map    `toml:"setting"`
	}
	StoreDriver interface {
		Connect(name string, config StoreConfig) (StoreConnect, error)
	}
	StoreConnect interface {
		Open() error
		Config() StoreConfig
		Health() (*StoreHealth, error)
		Close() error

		Base() StoreBase
	}

	StoreBase interface {
		Close() error
		Erred() error

		Upload(target string, metadatas ...Map) (File, Files)
		Download(code string) string
		Remove(code string)

		Browse(code string, name string, expiry ...time.Duration) string
		Preview(code string, w, h, t int64, expiry ...time.Duration) string
	}

	StoreHealth struct {
		Workload int64
	}
)

type (
	storeModule struct {
		driver coreBranch

		//文件配置，文件连接
		connects map[string]StoreConnect
		hashring *hashring.HashRing
	}
)

func (module *storeModule) Driver(name string, driver StoreDriver, overrides ...bool) {
	if driver == nil {
		panic("[存储]驱动不可为空")
	}

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	if override {
		module.driver.chunking(name, driver)
	} else {
		if module.driver.chunkdata(name) == nil {
			module.driver.chunking(name, driver)
		}
	}
}

func (module *storeModule) connecting(name string, config StoreConfig) (StoreConnect, error) {
	if driver, ok := module.driver.chunkdata(config.Driver).(StoreDriver); ok {
		return driver.Connect(name, config)
	}
	panic("[存储]不支持的驱动：" + config.Driver)
}

//初始化
func (module *storeModule) initing() {

	weights := make(map[string]int)
	for name, config := range Config.Store {
		if config.Weight > 0 {
			weights[name] = config.Weight
		}

		//连接
		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[存储]连接失败：" + err.Error())
		}

		//打开连接
		err = connect.Open()
		if err != nil {
			panic("[存储]打开失败：" + err.Error())
		}

		module.connects[name] = connect
	}

	module.hashring = hashring.New(weights)
}

//退出
func (module *storeModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

//返回文件Base对象
func (module *storeModule) Base(names ...string) StoreBase {
	name := DEFAULT
	if len(names) > 0 {
		name = names[0]
	} else {
		for key, _ := range module.connects {
			name = key
			break
		}
	}

	if connect, ok := module.connects[name]; ok {
		return connect.Base()
	}
	panic("[存储]无效存储连接")
}

//上传文件，是不是随便选一个库，还是选第一个库
func (module *storeModule) Upload(target string, metadata Map, bases ...string) (File, Files) {
	sb := module.Base(bases...)
	return sb.Upload(target, metadata)
}

//下载文件
func (module *storeModule) Download(code string) string {
	coding := Decoding(code)
	if coding == nil {
		return ""
	}

	sb := module.Base(coding.Base)
	return sb.Download(code)
}
