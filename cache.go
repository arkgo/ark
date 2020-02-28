package ark

import (
	"errors"
	"sync"
	"time"

	"github.com/arkgo/asset/hashring"
	. "github.com/arkgo/base"
)

type (
	// CacheConfig 缓存配置
	CacheConfig struct {
		Driver  string `toml:"driver"`
		Weight  int    `toml:"weight"`
		Prefix  string `toml:"prefix"`
		Setting Map    `toml:"setting"`
	}
	// CacheDriver 缓存驱动
	CacheDriver interface {
		Connect(string, CacheConfig) (CacheConnect, error)
	}

	// CacheHealth 缓存健康信息
	CacheHealth struct {
		Workload int64
	}

	// CacheHandler 缓存回调
	CacheHandler func(string, []byte) error

	// CacheConnect 缓存连接
	CacheConnect interface {
		Open() error
		Health() (CacheHealth, error)
		Close() error

		Read(string) (Any, error)
		Write(key string, val Any, exps ...time.Duration) error
		Delete(key string) error
		Stepping(key string, num int64) (int64, error)
		Keys(prefix string) ([]string, error)
		Clear(prefix string) error
	}

	cacheModule struct {
		mutex    sync.Mutex
		drivers  map[string]CacheDriver
		connects map[string]CacheConnect
		hashring *hashring.HashRing
	}
)

func newCache() *cacheModule {
	return &cacheModule{
		drivers:  make(map[string]CacheDriver, 0),
		connects: make(map[string]CacheConnect, 0),
	}
}

//注册缓存驱动
func (module *cacheModule) Driver(name string, driver CacheDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[缓存]驱动不可为空")
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

func (module *cacheModule) connecting(name string, config CacheConfig) (CacheConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(name, config)
	}
	panic("[日志]不支持的驱动" + config.Driver)
}
func (module *cacheModule) initing() {
	weights := make(map[string]int)
	for name, config := range ark.Config.Cache {
		weights[name] = config.Weight

		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[缓存]连接失败：" + err.Error())
		}
		err = connect.Open()
		if err != nil {
			panic("[缓存]打开失败：" + err.Error())
		}

		module.connects[name] = connect
	}

	//hashring分片
	module.hashring = hashring.New(weights)
}
func (module *cacheModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

func (module *cacheModule) Read(con, key string) (Any, error) {
	if con == "" && module.hashring != nil {
		con = module.hashring.Locate(key)
	}
	if connect, ok := module.connects[con]; ok {
		return connect.Read(key)
	}
	return nil, errors.New("读取缓存失败")
}

func (module *cacheModule) Write(con, key string, val Any, exp time.Duration) error {
	if con == "" && module.hashring != nil {
		con = module.hashring.Locate(key)
	}
	if connect, ok := module.connects[con]; ok {
		return connect.Write(key, val, exp)
	}
	return errors.New("写入缓存失败")
}

func (module *cacheModule) Delete(con, key string) error {
	if con == "" && module.hashring != nil {
		con = module.hashring.Locate(key)
	}
	if connect, ok := module.connects[con]; ok {
		return connect.Delete(key)
	}

	return errors.New("删除缓存失败")
}

func (module *cacheModule) Stepping(con, key string, step int64) (int64, error) {
	if con == "" && module.hashring != nil {
		con = module.hashring.Locate(key)
	}
	if connect, ok := module.connects[con]; ok {
		return connect.Stepping(key, step)
	}

	return int64(0), errors.New("删除缓存失败")
}

func (module *cacheModule) Keys(con, prefix string) ([]string, error) {
	if connect, ok := module.connects[con]; ok {
		return connect.Keys(prefix)
	}

	return nil, errors.New("清理缓存失败")
}

func (module *cacheModule) Clear(con, prefix string) error {
	if connect, ok := module.connects[con]; ok {
		return connect.Clear(prefix)
	}

	return errors.New("清理缓存失败")
}

func Cache(key string, vals ...Any) Any {
	if len(vals) > 0 {
		err := ark.Cache.Write("", key, vals[0], 0)
		if err == nil {
			return vals[0]
		}
		return nil
	} else {
		any, err := ark.Cache.Read("", key)
		if err != nil {
			return nil
		}
		return any
	}
}
func CacheKeys(prefix string, conns ...string) []string {
	con := DEFAULT
	if len(conns) > 0 {
		con = conns[0]
	}
	keys, err := ark.Cache.Keys(con, prefix)
	if err != nil {
		return []string{}
	}
	return keys
}

func CacheClear(prefix string, conns ...string) error {
	con := DEFAULT
	if len(conns) > 0 {
		con = conns[0]
	}
	return ark.Cache.Clear(con, prefix)
}

func Stepping(key string, step int64, conns ...string) int64 {
	con := "" //自动选项
	if len(conns) > 0 {
		con = conns[0]
	}
	num, err := ark.Cache.Stepping(con, key, step)
	if err != nil {
		return int64(0)
	}
	return num
}
