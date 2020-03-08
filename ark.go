package ark

import (
	"os"
	"os/signal"
	"syscall"

	. "github.com/arkgo/base"
)

type (

	//modules
	//node, codec, basic
	//gateway, service, website
	//logger, mutex
	//bus, file, store, cache, data
	//session, http, view

	arkCore struct {
		Config *arkConfig

		Node  *nodeModule
		Codec *codecModule
		Basic *basicModule

		Gateway *gatewayModule
		Service *serviceModule

		Logger *loggerModule
		Mutex  *mutexModule

		Bus   *busModule
		Store *storeModule
		Cache *cacheModule
		Data  *dataModule

		Session *sessionModule
		Http    *httpModule
		View    *viewModule

		readied, running bool
	}
	arkConfig struct {
		Name string `toml:"name"`
		Mode string `toml:"mode"`

		Node nodeConfig `toml:"node"`

		Basic basicConfig           `toml:"basic"`
		Lang  map[string]langConfig `toml:"lang"`

		Codec codecConfig `toml:"codec"`

		Logger LoggerConfig           `toml:"logger"`
		Mutex  map[string]MutexConfig `toml:"mutex"`

		Gateway GatewayConfig `toml:"gateway"`

		Bus   map[string]BusConfig   `toml:"bus"`
		File  FileConfig             `toml:"file"`
		Store map[string]StoreConfig `toml:"store"`
		Cache map[string]CacheConfig `toml:"cache"`
		Data  map[string]DataConfig  `toml:"data"`

		Session map[string]SessionConfig `toml:"session"`
		Http    HttpConfig               `toml:"http"`
		Site    map[string]SiteConfig    `toml:"site"`
		View    ViewConfig               `toml:"view"`

		hosts map[string]string

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
	ark.Http.initing()
	ark.View.initing()

	ark.readied = true
}
func (ark *arkCore) Start() {
	if ark.running {
		return
	}

	//需要监听端口什么的，就需要start，主要是http，node端口，啥的
	//因为有时候，会一些单独程序，需要连接库，但是不需要坚挺端口，比如，导入工具
	ark.Http.Start()

	ark.Logger.output("%s node %d started on %d", ark.Config.Name, ark.Config.Node.Id, ark.Config.Http.Port)
	ark.running = true
}
func (ark *arkCore) Waiting() {
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, os.Kill, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-exitChan
}
func (ark *arkCore) Stop() {

	ark.Logger.output("%s node %d stopped", ark.Config.Name, ark.Config.Node.Id)

	ark.View.exiting()
	ark.Http.exiting()
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
