package ark

import (
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/arkgo/asset"
	"github.com/arkgo/asset/cron"
	"github.com/arkgo/asset/hashring"
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

	// 总线回调
	EventHandler func(string, []byte) error
	QueueHandler func(string, []byte) error

	// BusConnect 总线连接
	BusConnect interface {
		Open() error
		Health() (BusHealth, error)
		Close() error

		Accept(EventHandler, QueueHandler) error

		Event(string) error
		Queue(string, int) error

		Start() error

		Publish(name string, data []byte, delays ...time.Duration) error
		Enqueue(name string, data []byte, delays ...time.Duration) error
	}

	busModule struct {
		mutex   sync.Mutex
		drivers map[string]BusDriver

		plans        map[string]Plan
		events       map[string]Event
		queues       map[string]Queue
		queueThreads map[string]int

		connects map[string]BusConnect
		hashring *hashring.HashRing

		cron        *cron.Cron
		cronEntries map[string][]string
	}

	Plan struct {
		Time   string   `json:"time"`
		Times  []string `json:"times"`
		Method string   `json:"method"`
		Value  Map      `json:"value"`

		Name     string   `json:"name"`
		Desc     string   `json:"desc"`
		Alias    []string `json:"alias"`
		Nullable bool     `json:"nullable"`
		Args     Vars     `json:"args"`
		Data     Vars     `json:"data"`
		Setting  Map      `json:"setting"`
		Action   Any      `json:"-"`
	}
	Event struct {
		Name     string   `json:"name"`
		Desc     string   `json:"desc"`
		Alias    []string `json:"alias"`
		Nullable bool     `json:"nullable"`
		Args     Vars     `json:"args"`
		Data     Vars     `json:"data"`
		Setting  Map      `json:"setting"`
		Action   Any      `json:"-"`
	}
	Queue struct {
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Alias  []string `json:"alias"`
		Thread int      `json:"thread"`
	}
)

func newBus() *busModule {
	return &busModule{
		drivers: make(map[string]BusDriver, 0),

		plans:        make(map[string]Plan),
		events:       make(map[string]Event),
		queues:       make(map[string]Queue),
		queueThreads: make(map[string]int),

		connects: make(map[string]BusConnect, 0),
	}
}

//注册总线驱动
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

func (module *busModule) Plan(name string, config Plan, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	if config.Times == nil {
		config.Times = make([]string, 0)
	}
	if config.Time != "" {
		config.Times = append(config.Times, config.Time)
		config.Time = ""
	}

	if override {
		module.plan(name, config)
	} else {
		if _, ok := module.events[name]; ok == false {
			module.plan(name, config)
		}
	}
}
func (module *busModule) plan(name string, config Plan) {
	if config.Method == "" {
		config.Method = name //调用自己
	}
	if config.Action != nil {
		//如果action不为空，代注册方法
		ark.Service.Method(name, Method{
			Name: config.Name, Desc: config.Desc, Alias: config.Alias,
			Nullable: config.Nullable, Args: config.Args, Data: config.Data,
			Setting: config.Setting, Action: config.Action,
		})
	}

	module.plans[name] = config
}

func (module *busModule) Event(name string, config Event, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	if override {
		module.event(name, config)
	} else {
		if _, ok := module.events[name]; ok == false {
			module.event(name, config)
		}
	}
}
func (module *busModule) event(name string, config Event) {
	if config.Action != nil {
		//如果action不为空，代注册方法
		ark.Service.Method(name, Method{
			Name: config.Name, Desc: config.Desc, Alias: config.Alias,
			Nullable: config.Nullable, Args: config.Args, Data: config.Data,
			Setting: config.Setting, Action: config.Action,
		})
	}

	module.events[name] = config
}

func (module *busModule) Queue(name string, config Queue, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	alias := make([]string, 0)
	if name != "" {
		alias = append(alias, name)
	}
	if config.Alias != nil {
		alias = append(alias, config.Alias...)
	}

	if config.Thread <= 1 {
		config.Thread = 1
	}

	for _, key := range alias {
		if override {
			module.queues[key] = config
			module.queueThreads[key] = config.Thread
		} else {
			if _, ok := module.queues[key]; ok == false {
				module.queues[key] = config
				module.queueThreads[key] = config.Thread
			}
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

	//开始计划
	module.cron = cron.New()
	module.cronEntries = make(map[string][]string)

	for key, val := range module.plans {
		name := key
		config := val

		ids := make([]string, 0)
		for i, crontab := range config.Times {
			timeName := fmt.Sprintf("%s.%v", key, i)
			id, err := module.cron.AddFunc(crontab, func() {

				//超时，那边要判断是不是主节点
				module.planning(name, config)

			}, &cron.Extra{Name: timeName, RunForce: false, TimeOut: 5})

			if err != nil {
				panic("[总线]注册计划失败")
			}

			ids = append(ids, id)
		}

		module.cronEntries[name] = ids
	}

	module.cron.Start()

	//-----------------------注册事件和队列--------------------

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

		err = connect.Accept(module.eventing, module.queueing)
		if err != nil {
			panic("[总线]注册失败：" + err.Error())
		}

		//权重大于0，才表示是本系统自动要使用的消息服务
		//如果小于等于0，则表示是外接的消息系统，就不订阅
		if config.Weight > 0 {
			weights[name] = config.Weight

			//待处理，注册订阅和队列

			for name, _ := range module.events {
				if err := connect.Event(name); err != nil {
					panic("[总线]注册事件失败：" + err.Error())
				}
			}

			for name, thread := range module.queueThreads {
				if err := connect.Queue(name, thread); err != nil {
					panic("[总线]注册队列失败：" + err.Error())
				}
			}
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
	if module.cron != nil {
		module.cron.Stop()
	}
	for _, connect := range module.connects {
		connect.Close()
	}
}

//收到计划
func (module *busModule) planning(name string, config Plan) {
	ark.Service.Invoke(nil, config.Method, config.Value)
}

//收到事件和队列
func (module *busModule) eventing(name string, data []byte) error {
	value := Map{}
	err := ark.Codec.Unmarshal(data, &value)
	if err == nil {
		ark.Service.Invoke(nil, name, value)
	}

	return nil
}
func (module *busModule) queueing(name string, data []byte) error {
	// msg := BusMessage{}
	// err := json.Unmarshal(data, &msg)
	// if err == nil {
	// 	_, res := mService.Execute(msg.Name, msg.Value)
	// 	if res == Retry {
	// 		//如果是结果是重试，重新加入队列
	// 		Enqueue(msg.Name, msg.Value)
	// 	}
	// }

	value := Map{}
	err := ark.Codec.Unmarshal(data, &value)
	if err == nil {
		ark.Service.Invoke(nil, name, value)
	}

	return nil
}

// Publish 发起事件
func (module *busModule) Publish(name string, value Map, delays ...time.Duration) error {
	//待优化，可能使用其它方式来编码
	if value == nil {
		value = Map{}
	}
	data, err := ark.Codec.Marshal(value)
	if err != nil {
		return err
	}

	//使用权重来发决定，使用哪一条总线
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(name)
	}
	if connect, ok := module.connects[locate]; ok {
		return connect.Publish(name, data, delays...)
	}
	return errors.New("发布失败")
}

// Enqueue 发起队列
func (module *busModule) Enqueue(name string, value Map, delays ...time.Duration) error {
	//待优化，可能使用其它方式来编码
	if value == nil {
		value = Map{}
	}
	data, err := ark.Codec.Marshal(value)
	if err != nil {
		return err
	}

	//使用权重来发消息
	locate := DEFAULT
	if module.hashring != nil {
		locate = module.hashring.Locate(name)
	}
	if connect, ok := module.connects[locate]; ok {
		return connect.Enqueue(name, data, delays...)
	}

	return errors.New("列队失败")
}

func (module *busModule) PublishTo(bus string, name string, data []byte, delays ...time.Duration) error {
	if connect, ok := module.connects[bus]; ok {
		return connect.Publish(name, data, delays...)
	}

	return errors.New("发布失败")
}

func (module *busModule) EnqueueTo(bus string, name string, data []byte, delays ...time.Duration) error {
	if connect, ok := module.connects[bus]; ok {
		return connect.Enqueue(name, data, delays...)
	}

	return errors.New("列队失败")
}

//--------------------------------------------

// Event 注册事件
// func Event(name string, config Map, overrides ...bool) {
// 	ark.Bus.Event(name, config, overrides...)
// }
// func Queue(name string, config Map, overrides ...bool) {
// 	ark.Bus.Queue(name, config, overrides...)
// }

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
	data, err := ark.Codec.Marshal(value)
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
	data, err := ark.Codec.Marshal(value)
	if err != nil {
		return err
	}
	return ark.Bus.EnqueueTo(bus, name, data, delays...)
}

// EnqueueDataTo 发起原始外部队列
func EnqueueDataTo(bus string, name string, data []byte, delays ...time.Duration) error {
	return ark.Bus.EnqueueTo(bus, name, data, delays...)
}

// Trigger 触发器，待处理
// func Trigger(name string, values ...Map) error {
// 	return nil
// }
