package ark

import (
	"errors"
	"sync"
	"time"

	. "github.com/arkgo/asset"
	"github.com/arkgo/asset/hashring"
)

type (

	// MutexConfig 是互斥配置类
	MutexConfig struct {
		Driver  string `toml:"driver"`
		Weight  int    `toml:"weight"`
		Prefix  string `toml:"prefix"`
		Expiry  string `toml:"expiry"`
		Setting Map    `toml:"setting"`
	}

	// MutexDriver 互斥驱动
	MutexDriver interface {
		Connect(name string, config MutexConfig) (MutexConnect, error)
	}
	// MutexConnect 互斥连接
	MutexConnect interface {
		//打开、关闭
		Open() error
		Health() (MutexHealth, error)
		Close() error

		Lock(key string, expiries ...time.Duration) error
		Unlock(key string) error
	}

	// MutexHealth 互斥健康信息
	MutexHealth struct {
		Workload int64
	}

	mutexModule struct {
		mutex   sync.Mutex
		drivers map[string]MutexDriver

		connects map[string]MutexConnect
		weights  map[string]int
		hashring *hashring.HashRing
	}
)

func newMutex() *mutexModule {
	return &mutexModule{
		drivers:  make(map[string]MutexDriver),
		connects: make(map[string]MutexConnect),
	}
}

//注册互斥驱动
func (module *mutexModule) Driver(name string, driver MutexDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[互斥]驱动不可为空")
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

func (module *mutexModule) connecting(name string, config MutexConfig) (MutexConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(name, config)
	}
	panic("[互斥]不支持的驱动" + config.Driver)
}

//初始化
func (module *mutexModule) initing() {

	weights := make(map[string]int)
	for name, config := range ark.Config.Mutex {
		if config.Weight > 0 {
			//只有设置了权重的才参与分布
			weights[name] = config.Weight
		}

		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[互斥]连接失败：" + err.Error())
		}

		//打开连接
		err = connect.Open()
		if err != nil {
			panic("[互斥]打开失败：" + err.Error())
		}

		//保存连接
		module.connects[name] = connect
	}

	//hashring分片
	module.weights = weights
	module.hashring = hashring.New(weights)
}

//退出
func (module *mutexModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

func (module *mutexModule) Lock(key string, expiry time.Duration, cons ...string) error {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	expiries := make([]time.Duration, 0)
	if expiry > 0 {
		expiries = append(expiries, expiry)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Lock(key, expiries...)
	}

	return errors.New("无效互斥连接")
}
func (module *mutexModule) Unlock(key string, cons ...string) error {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Unlock(key)
	}

	return errors.New("无效互斥连接")
}

//语法糖
func Locked(key string, expiry time.Duration, cons ...string) bool {
	return ark.Mutex.Lock(key, expiry, cons...) != nil
}
func Lock(key string, expiry time.Duration, cons ...string) error {
	return ark.Mutex.Lock(key, expiry, cons...)
}
func Unlock(key string, cons ...string) error {
	return ark.Mutex.Unlock(key, cons...)
}
