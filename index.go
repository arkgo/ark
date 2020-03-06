package ark

import (
	"math"

	. "github.com/arkgo/base"
)

var (
	ark     *arkCore
	Mode    Env
	Setting Map

	Sites *httpSite
	Root  *httpSite
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

func Precision(f float64, prec int, rounds ...bool) float64 {
	round := false
	if len(rounds) > 0 {
		round = rounds[0]
	}

	pow10_n := math.Pow10(prec)
	if round {
		//四舍五入
		return math.Trunc((f+0.5/pow10_n)*pow10_n) / pow10_n
	}
	//默认
	return math.Trunc((f)*pow10_n) / pow10_n
}
