package ark

import (
	"errors"
	"sync"
	"time"

	"github.com/arkgo/asset/util"
	. "github.com/arkgo/base"
)

type (

	// MutexConfig 是互斥配置类
	MutexConfig struct {
		Driver  string `toml:"driver"`
		Prefix  string `toml:"prefix"`
		Expiry  string `toml:"expiry"`
		Setting Map    `toml:"setting"`
	}

	// MutexDriver 互斥驱动
	MutexDriver interface {
		Connect(config MutexConfig) (MutexConnect, error)
	}
	// MutexConnect 互斥连接
	MutexConnect interface {
		//打开、关闭
		Open() error
		Health() (MutexHealth, error)
		Close() error

		Lock(key string, expiriy time.Duration) error
		Unlock(key string) error
	}

	// MutexHealth 互斥健康信息
	MutexHealth struct {
		Workload int64
	}

	mutexModule struct {
		mutex   sync.Mutex
		drivers map[string]MutexDriver

		connect MutexConnect
		expiry  time.Duration
	}
)

func newMutex() *mutexModule {
	expiry := time.Second * 2
	if ark.Config.Mutex.Expiry != "" {
		if d, err := util.ParseDuration(ark.Config.Mutex.Expiry); err == nil {
			expiry = d
		}
	}

	return &mutexModule{
		drivers: map[string]MutexDriver{},
		expiry:  expiry,
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

func (module *mutexModule) connecting(config MutexConfig) (MutexConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(config)
	}
	panic("[互斥]不支持的驱动" + config.Driver)
}

//初始化
func (module *mutexModule) initing() {
	connect, err := module.connecting(ark.Config.Mutex)
	if err != nil {
		panic("[互斥]连接失败：" + err.Error())
	}

	//打开连接
	err = connect.Open()
	if err != nil {
		panic("[互斥]打开失败：" + err.Error())
	}

	//保存连接
	module.connect = connect
}

//退出
func (module *mutexModule) exiting() {
	if module.connect != nil {
		module.connect.Close()
	}
}

func (module *mutexModule) Lock(key string, expiries ...time.Duration) error {
	if module.connect != nil {
		expiry := module.expiry
		if len(expiries) > 0 {
			expiry = expiries[0]
		}
		return module.Lock(key, expiry)
	}
	return errors.New("无效互斥连接")
}
func (module *mutexModule) Unlock(key string) error {
	if module.connect != nil {
		return module.Unlock(key)
	}
	return errors.New("无效互斥连接")
}

//语法糖

func Lock(key string, expiries ...time.Duration) error {
	return ark.Mutex.Lock(key, expiries...)
}
func Unlock(key string) error {
	return ark.Mutex.Unlock(key)
}
