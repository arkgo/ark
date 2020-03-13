package ark

import (
	"fmt"
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

// func Register(name string, data Any, overrides ...bool) {
// 	switch config := data.(type) {
// 	case eventRegister:
// 		ark.Bus.Event(name, config, overrides...)
// 	}
// }

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

// Register 注册中心
func Register(args ...Any) {
	var key string = ""
	var value Any
	var override bool = true

	if len(args) == 0 {
		panic("[注册]无效注册参数")
	} else if len(args) == 1 {
		value = args[0]
	} else if len(args) == 2 {
		if vv, ok := args[0].(string); ok {
			//name, value
			key = vv
			value = args[1]
		} else if vv, ok := args[1].(bool); ok {
			//value, override
			value = args[0]
			override = vv
		} else {

		}
	} else {
		if vv, ok := args[0].(string); ok {
			key = vv
		}
		value = args[1]
		if vv, ok := args[2].(bool); ok {
			override = vv
		}
	}

	switch val := value.(type) {
	case State:
		ark.Basic.State(key, val, override)
	case Mime:
		ark.Basic.Mime(key, val, override)
	case Regular:
		ark.Basic.Regular(key, val, override)
	case Type:
		ark.Basic.Type(key, val, override)
	case Crypto:
		ark.Basic.Crypto(key, val, override)

	case Filter:
		ark.Http.Filter(key, val, override)
	case RequestFilter:
		ark.Http.RequestFilter(key, val, override)
	case ExecuteFilter:
		ark.Http.ExecuteFilter(key, val, override)
	case FoundHandler:
		ark.Http.FoundHandler(key, val, override)
	case ErrorHandler:
		ark.Http.ErrorHandler(key, val, override)
	case FailedHandler:
		ark.Http.FailedHandler(key, val, override)
	case DeniedHandler:
		ark.Http.DeniedHandler(key, val, override)

	case Helper:
		ark.View.Helper(key, val, override)
	}

}

// Register 注册中心
func (site *httpSite) Register(name string, value Any, overrides ...bool) {
	key := fmt.Sprintf("%s.%s", site.name, name)

	switch val := value.(type) {
	case Filter:
		ark.Http.Filter(key, val, overrides...)
	case RequestFilter:
		ark.Http.RequestFilter(key, val, overrides...)
	case ExecuteFilter:
		ark.Http.ExecuteFilter(key, val, overrides...)
	case FoundHandler:
		ark.Http.FoundHandler(key, val, overrides...)
	case ErrorHandler:
		ark.Http.ErrorHandler(key, val, overrides...)
	case FailedHandler:
		ark.Http.FailedHandler(key, val, overrides...)
	case DeniedHandler:
		ark.Http.DeniedHandler(key, val, overrides...)
	}

}
