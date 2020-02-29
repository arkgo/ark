package ark

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/arkgo/asset/hashring"
	. "github.com/arkgo/base"
)

type (
	// BusConfig 总线配置
	BusConfig struct {
		Driver  string `toml:"driver"`
		Weight  int    `toml:"weight"`
		Prefix  string `toml:"prefix"`
		Setting Map    `toml:"setting"`
	}
	// BusDriver 总线驱动
	BusDriver interface {
		Connect(string, BusConfig) (BusConnect, error)
	}

	// BusHealth 总线健康信息
	BusHealth struct {
		Workload int64
	}

	// BusHandler 总线回调
	BusHandler func(string, []byte) error

	// BusConnect 总线连接
	BusConnect interface {
		Open() error
		Health() (BusHealth, error)
		Close() error
		Start() error

		Event(string, BusHandler) error
		Queue(string, int, BusHandler) error

		Publish(name string, data []byte) error
		DeferredPublish(name string, data []byte, delay time.Duration) error
		Enqueue(name string, data []byte) error
		DeferredEnqueue(name string, data []byte, delay time.Duration) error
	}

	busModule struct {
		mutex    sync.Mutex
		drivers  map[string]BusDriver
		connects map[string]BusConnect
		hashring *hashring.HashRing
	}
)

func newBus() *busModule {
	return &busModule{
		drivers:  make(map[string]BusDriver, 0),
		connects: make(map[string]BusConnect, 0),
	}
}

//注册日志驱动
func (module *busModule) Driver(name string, driver BusDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[总线]驱动不可为空")
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

func (module *busModule) connecting(name string, config BusConfig) (BusConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(name, config)
	}
	panic("[总线]不支持的驱动" + config.Driver)
}
func (module *busModule) initing() {
	weights := make(map[string]int)
	for name, config := range ark.Config.Bus {

		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[总线]连接失败：" + err.Error())
		}
		err = connect.Open()
		if err != nil {
			panic("[总线]打开失败：" + err.Error())
		}

		//权重大于0，才表示是本系统自动要使用的消息服务
		//如果小于等于0，则表示是外接的消息系统，就不订阅
		if config.Weight > 0 {
			weights[name] = config.Weight

			//待处理，注册订阅和队列

			// connect.Event(busBroadcast, module.broadcast)
			// connect.Event(busMulticast, module.multicast)
			// connect.Event(busUnicast, module.unicast)
			// connect.Event(busDegrade, module.degrade)

			// connect.Event(busEvent, module.event)

			//200221更新，不再注册所有队列
			//队列全部走liner里去监听，队列全部单独注册和发送
			//connect.Queue(busQueue, 0, module.queue)	//此为通用的队列接收

			//单独处理指定了liner的队列
			// for _, liner := range liners {
			// 	if line, ok := liner.data.(int); ok {
			// 		connect.Queue(liner.name, line, module.queue)
			// 	}
			// }
		}

		err = connect.Start()
		if err != nil {
			panic("[总线]启动失败：" + err.Error())
		}

		module.connects[name] = connect
	}

	//hashring分片
	module.hashring = hashring.New(weights)
}
func (module *busModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

// Publish 发起事件
func (module *busModule) Publish(name string, value Map, delays ...time.Duration) error {
	//待优化，可能使用其它方式来编码
	if value == nil {
		value = Map{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	//使用权重来发决定，使用哪一条总线
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(name)
	}
	if connect, ok := module.connects[locate]; ok {
		if len(delays) > 0 {
			return connect.DeferredPublish(name, data, delays[0])
		}
		return connect.Publish(name, data)
	}
	return errors.New("发布失败")
}

// Enqueue 发起队列
func (module *busModule) Enqueue(name string, value Map, delays ...time.Duration) error {
	//待优化，可能使用其它方式来编码
	if value == nil {
		value = Map{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	//使用权重来发消息
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(name)
	}
	if connect, ok := module.connects[locate]; ok {
		if len(delays) > 0 {
			return connect.DeferredEnqueue(name, data, delays[0])
		}
		return connect.Enqueue(name, data)
	}

	return errors.New("列队失败")
}

func (module *busModule) PublishTo(bus string, name string, data []byte, delays ...time.Duration) error {
	if connect, ok := module.connects[bus]; ok {
		if len(delays) > 0 {
			return connect.DeferredPublish(name, data, delays[0])
		}
		return connect.Publish(name, data)
	}

	return errors.New("发布失败")
}

func (module *busModule) EnqueueTo(bus string, name string, data []byte, delays ...time.Duration) error {
	if connect, ok := module.connects[bus]; ok {
		if len(delays) > 0 {
			return connect.DeferredEnqueue(name, data, delays[0])
		}
		return connect.Enqueue(name, data)
	}

	return errors.New("列队失败")
}

// Publish 是发起事件
func Publish(name string, value Map, delays ...time.Duration) error {
	return ark.Bus.Publish(name, value, delays...)
}

// PublishTo 发起外部事件
func PublishTo(bus string, name string, value Map, delays ...time.Duration) error {
	//待优化，可能使用其它方式来编码
	if value == nil {
		value = Map{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ark.Bus.PublishTo(bus, name, data, delays...)
}

// PublishDataTo 发起原始外部事件
func PublishDataTo(bus string, name string, data []byte, delays ...time.Duration) error {
	return ark.Bus.PublishTo(bus, name, data, delays...)
}

// Enqueue 是发起队列
func Enqueue(name string, value Map, delays ...time.Duration) error {
	return ark.Bus.Enqueue(name, value, delays...)
}

// EnqueueTo 发起外部队列
func EnqueueTo(bus string, name string, value Map, delays ...time.Duration) error {
	//待优化，可能使用其它方式来编码
	if value == nil {
		value = Map{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ark.Bus.EnqueueTo(bus, name, data, delays...)
}

// EnqueueDataTo 发起原始外部队列
func EnqueueDataTo(bus string, name string, data []byte, delays ...time.Duration) error {
	return ark.Bus.EnqueueTo(bus, name, data, delays...)
}
