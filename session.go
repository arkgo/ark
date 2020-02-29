package ark

import (
	"errors"
	"sync"
	"time"

	"github.com/arkgo/asset/hashring"
	. "github.com/arkgo/base"
)

type (
	// SessionConfig 会话配置
	SessionConfig struct {
		Driver  string `toml:"driver"`
		Weight  int    `toml:"weight"`
		Prefix  string `toml:"prefix"`
		Expiry  string `toml:"expiry"`
		Setting Map    `toml:"setting"`
	}
	// SessionDriver 会话驱动
	SessionDriver interface {
		Connect(string, SessionConfig) (SessionConnect, error)
	}

	// SessionHealth 会话健康信息
	SessionHealth struct {
		Workload int64
	}

	// SessionConnect 会话连接
	SessionConnect interface {
		Open() error
		Health() (SessionHealth, error)
		Close() error

		Read(id string) (Map, error)
		Write(id string, value Map, expiries ...time.Duration) error
		Delete(id string) error
		Clear() error
	}

	sessionModule struct {
		mutex    sync.Mutex
		drivers  map[string]SessionDriver
		connects map[string]SessionConnect
		hashring *hashring.HashRing
	}
)

func newSession() *sessionModule {
	return &sessionModule{
		drivers:  make(map[string]SessionDriver, 0),
		connects: make(map[string]SessionConnect, 0),
	}
}

//注册会话驱动
func (module *sessionModule) Driver(name string, driver SessionDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[会话]驱动不可为空")
	}

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	if override {
		module.drivers[name] = driver
	} else {
		if module.drivers[name] == nil {
			module.drivers[name] = driver
		}
	}
}

func (module *sessionModule) connecting(name string, config SessionConfig) (SessionConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(name, config)
	}
	panic("[会话]不支持的驱动" + config.Driver)
}
func (module *sessionModule) initing() {
	weights := make(map[string]int)
	for name, config := range ark.Config.Session {
		weights[name] = config.Weight

		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[会话]连接失败：" + err.Error())
		}
		err = connect.Open()
		if err != nil {
			panic("[会话]打开失败：" + err.Error())
		}

		module.connects[name] = connect
	}

	//hashring分片
	module.hashring = hashring.New(weights)
}
func (module *sessionModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

func (module *sessionModule) Read(id string) (Map, error) {
	//使用权重来发消息
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(id)
	}
	if connect, ok := module.connects[locate]; ok {
		return connect.Read(id)
	}

	return Map{}, errors.New("读取会话失败")

}

func (module *sessionModule) Write(id string, value Map, expiry time.Duration) error {
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(id)
	}
	if connect, ok := module.connects[locate]; ok {
		return connect.Write(id, value, expiry)
	}

	return errors.New("写入会话失败")
}

func (module *sessionModule) Delete(id string) error {
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(id)
	}
	if connect, ok := module.connects[locate]; ok {
		return connect.Delete(id)
	}

	return errors.New("删除会话失败")
}

func (module *sessionModule) Clear(cccs ...string) error {
	name := DEFAULT
	if len(cccs) > 0 {
		name = cccs[0]
	}

	if connect, ok := module.connects[name]; ok {
		return connect.Clear()
	}

	return errors.New("清空会话失败")
}
