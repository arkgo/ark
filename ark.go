package ark

import (
	"os"
	"os/signal"
	"syscall"

	. "github.com/arkgo/base"
)

type (
	arkCore struct {
		Config *arkConfig

		Node   *nodeModule
		Serial *serialModule
		Basic  *basicModule

		Logger *loggerModule
		Mutex  *mutexModule

		Bus   *busModule
		Store *storeModule
		Cache *cacheModule
		Data  *dataModule

		Session *sessionModule

		readied, running bool
	}
	arkConfig struct {
		Name string `toml:"name"`
		Mode string `toml:"mode"`

		Node nodeConfig `toml:"node"`

		Basic basicConfig           `toml:"basic"`
		Lang  map[string]langConfig `toml:"lang"`

		Serial serialConfig `toml:"serial"`

		Logger LoggerConfig `toml:"logger"`
		Mutex  MutexConfig  `toml:"mutex"`

		Bus   map[string]BusConfig   `toml:"bus"`
		File  FileConfig             `toml:"file"`
		Store map[string]StoreConfig `toml:"store"`

		Session map[string]SessionConfig `toml:"session"`
		Cache   map[string]CacheConfig   `toml:"cache"`
		Data    map[string]DataConfig    `toml:"data"`

		Setting Map `toml:"setting"`
	}
)

func (ark *arkCore) Ready() {
	if ark.readied {
		return
	}

	ark.Logger.initing()
	ark.Mutex.initing()

	ark.Bus.initing()
	ark.Store.initing()
	ark.Cache.initing()
	ark.Data.initing()
	ark.Session.initing()

	ark.readied = true
}
func (ark *arkCore) Start() {
	if ark.running {
		return
	}

	//需要监听端口什么的，就需要start，主要是http，node端口，啥的
	//因为有时候，会一些单独程序，需要连接库，但是不需要坚挺端口，比如，导入工具

	ark.Logger.output("%s node %d started", ark.Config.Name, ark.Config.Node.Id)
	ark.running = true
}
func (ark *arkCore) Waiting() {
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, os.Kill, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-exitChan
}
func (ark *arkCore) Stop() {

	ark.Logger.output("%s node %d stopped", ark.Config.Name, ark.Config.Node.Id)

	ark.Session.exiting()
	ark.Data.exiting()
	ark.Cache.exiting()

	ark.Store.exiting()
	ark.Bus.exiting()

	ark.Mutex.exiting()
	ark.Logger.exiting()
}

func (ark *arkCore) Go() {
	ark.Ready()
	ark.Start()
	ark.Waiting()
	ark.Stop()
}

func Ready() {
	ark.Ready()
}
func Start() {
	ark.Start()
}
func Stop() {
	ark.Stop()
}
func Go() {
	ark.Go()
}

func Driver(name string, driver Any) {
	switch drv := driver.(type) {
	case LoggerDriver:
		ark.Logger.Driver(name, drv)
	case MutexDriver:
		ark.Mutex.Driver(name, drv)
	case BusDriver:
		ark.Bus.Driver(name, drv)
	case StoreDriver:
		ark.Store.Driver(name, drv)
	case SessionDriver:
		ark.Session.Driver(name, drv)
	case CacheDriver:
		ark.Cache.Driver(name, drv)
	case DataDriver:
		ark.Data.Driver(name, drv)
	}
}
