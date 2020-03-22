package ark

import (
	"math"

	. "github.com/arkgo/asset"
)

var (
	ark     *arkCore
	Mode    Env
	Setting Map

	Sites *httpSite
	Root  *httpSite
)

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

	case LoggerDriver:
		ark.Logger.Driver(key, val)
	case MutexDriver:
		ark.Mutex.Driver(key, val)
	case BusDriver:
		ark.Bus.Driver(key, val)
	case StoreDriver:
		ark.Store.Driver(key, val)
	case SessionDriver:
		ark.Session.Driver(key, val)
	case CacheDriver:
		ark.Cache.Driver(key, val)
	case DataDriver:
		ark.Data.Driver(key, val)
	case HttpDriver:
		ark.Http.Driver(key, val)
	case ViewDriver:
		ark.View.Driver(key, val)

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

	case Plan:
		ark.Bus.Plan(key, val, override)
	case Event:
		ark.Bus.Event(key, val, override)
	case Queue:
		ark.Bus.Queue(key, val, override)

	case Table:
		ark.Data.Table(key, val, override)
	case View:
		ark.Data.View(key, val, override)
	case Model:
		ark.Data.Model(key, val, override)

	case Method:
		ark.Service.Method(key, val, override)

	case Router:
		ark.Http.Router(key, val, override)

	case Filter:
		ark.Http.Filter(key, val, override)
	case RequestFilter:
		ark.Http.RequestFilter(key, val, override)
	case ExecuteFilter:
		ark.Http.ExecuteFilter(key, val, override)
	case ResponseFilter:
		ark.Http.ResponseFilter(key, val, override)
	case Handler:
		ark.Http.Handler(key, val, override)
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

func Define(tttt string, require bool, name string, extends ...Map) Var {
	config := Var{
		Type: tttt, Require: require, Name: name,
	}

	if len(extends) > 0 {
		extend := extends[0]

		if vv, ok := extend["default"]; ok {
			config.Default = vv
		}
		if vv, ok := extend["children"].(Vars); ok {
			config.Children = vv
		}
		if vv, ok := extend["setting"].(Map); ok {
			config.Setting = vv
		}
	}

	return config
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

func InvokingConfig(offset, limit int64, extends ...Vars) Vars {
	config := Vars{
		"offset": Var{
			Type: "int", Require: true, Default: offset, Name: "offset", Desc: "offset",
		},
		"limit": Var{
			Type: "int", Require: true, Default: limit, Name: "limit", Desc: "limit",
		},
	}

	return VarExtend(config, extends...)
}

func VarExtend(config Vars, extends ...Vars) Vars {
	if len(extends) > 0 {
		for k, v := range extends[0] {
			if v.Nil() {
				delete(config, k)
			} else {
				config[k] = v
			}
		}
	}
	return config
}
