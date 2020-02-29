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
		Expiry  string `toml:"expiry"`
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
		Exists(key string) (bool, error)
		Delete(key string) error
		Serial(key string, step int64) (int64, error)
		Keys(prefix ...string) ([]string, error)
		Clear(prefix ...string) error
	}

	cacheModule struct {
		mutex    sync.Mutex
		drivers  map[string]CacheDriver
		connects map[string]CacheConnect
		weights  map[string]int
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
	panic("[缓存]不支持的驱动" + config.Driver)
}
func (module *cacheModule) initing() {
	weights := make(map[string]int)
	for name, config := range ark.Config.Cache {
		if config.Weight > 0 {
			//只有设置了权重的缓存才参与分布
			weights[name] = config.Weight
		}

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
	module.weights = weights
	module.hashring = hashring.New(weights)
}
func (module *cacheModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

func (module *cacheModule) Read(key string, cons ...string) (Any, error) {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Read(key)
	}
	return nil, errors.New("读取缓存失败")
}

func (module *cacheModule) Exists(key string, cons ...string) (bool, error) {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Exists(key)
	}
	return false, errors.New("读取缓存失败")
}

func (module *cacheModule) Write(key string, val Any, exp time.Duration, cons ...string) error {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Write(key, val, exp)
	}
	return errors.New("写入缓存失败")
}

func (module *cacheModule) Delete(key string, cons ...string) error {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Delete(key)
	}

	return errors.New("删除缓存失败")
}

func (module *cacheModule) Serial(key string, step int64, cons ...string) (int64, error) {
	con := DEFAULT
	if len(cons) > 0 && cons[0] != "" {
		con = cons[0]
	} else if module.hashring != nil {
		con = module.hashring.Locate(key)
	}

	if connect, ok := module.connects[con]; ok {
		return connect.Serial(key, step)
	}

	return int64(0), errors.New("删除缓存失败")
}

func (module *cacheModule) Keys(prefix string, cons ...string) ([]string, error) {

	//如果未指定连接，就清理所有参与分布的
	if len(cons) > 0 {
		con := cons[0]
		if connect, ok := module.connects[con]; ok {
			return connect.Keys(prefix)
		}
	} else {
		keys := []string{}
		for k, _ := range module.weights {
			if connect, ok := module.connects[k]; ok {
				ks, e := connect.Keys(prefix)
				if e == nil {
					keys = append(keys, ks...)
				}
			}
		}
		return keys, nil
	}
	return nil, errors.New("获取缓存键失败")
}

func (module *cacheModule) Clear(prefix string, cons ...string) error {
	//如果未指定连接，就清理所有参与分布的
	if len(cons) > 0 {
		con := cons[0]
		if connect, ok := module.connects[con]; ok {
			return connect.Clear(prefix)
		}
	} else {
		for k, _ := range module.weights {
			if connect, ok := module.connects[k]; ok {
				connect.Clear(prefix)
			}
		}
	}
	return nil
}

func Cache(key string, vals ...Any) Any {
	if len(vals) > 0 {
		val := vals[0]
		if val == nil {
			//空值就是删除啦
			ark.Cache.Delete(key)
			return nil
		} else {
			err := ark.Cache.Write(key, val, 0)
			if err == nil {
				return val
			}
			return nil
		}
	} else {
		any, err := ark.Cache.Read(key)
		if err != nil {
			return nil
		}
		return any
	}
}
func Cached(key string, cons ...string) bool {
	yes, err := ark.Cache.Exists(key, cons...)
	if err != nil {
		return false
	}
	return yes
}

func CacheKeys(prefix string, cons ...string) []string {
	keys, err := ark.Cache.Keys(prefix, cons...)
	if err != nil {
		return []string{}
	}
	return keys
}

func CacheClear(prefix string, cons ...string) error {
	return ark.Cache.Clear(prefix, cons...)
}

func Sequence(key string, step int64, cons ...string) int64 {
	num, err := ark.Cache.Serial(key, step, cons...)
	if err != nil {
		return int64(0)
	}
	return num
}
