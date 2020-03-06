package ark

import (
	. "github.com/arkgo/base"
)

var (
	ark     *arkCore
	Mode    Env
	Setting Map
)

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
	case HttpDriver:
		ark.Http.Driver(name, drv)
	case ViewDriver:
		ark.View.Driver(name, drv)
	}
}
