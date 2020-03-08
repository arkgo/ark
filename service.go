package ark

import (
	"fmt"
	"sync"

	. "github.com/arkgo/base"
)

type (
	serviceModule struct {
		mutex   sync.Mutex
		methods map[string]Map
	}

	serviceLibrary struct {
		module *serviceModule
		name   string
	}

	serviceLogic struct {
		ctx     context
		Name    string
		Setting Map
	}

	Service struct {
		ctx     context
		Name    string
		Config  Map
		Setting Map
		Value   Map
		Args    Map
	}
)

func newService() *serviceModule {
	return &serviceModule{
		methods: make(map[string]Map, 0),
	}
}

//注册方法
func (module *serviceModule) Method(name string, config Map, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if config == nil {
		panic("[服务]配置不可为空")
	}

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	if override {
		module.methods[name] = config
	} else {
		if module.methods[name] == nil {
			module.methods[name] = config
		}
	}
}

func (module *serviceModule) Invoke(ctx context, name string, value Map, settings ...Map) (Map, *Res) {
	config := make(Map)
	if vv, ok := module.methods[name]; ok == false {
		return nil, Fail
	} else {
		for k, v := range vv {
			config[k] = v
		}
	}

	var setting Map
	if len(settings) > 0 {
		setting = settings[0]
	}

	if ctx == nil {
		ctx = emptyContext()
		defer ctx.terminal()
	}
	if value == nil {
		value = make(Map)
	}
	if setting == nil {
		setting = make(Map)
	}

	argn := false
	if v, ok := config["argn"].(bool); ok {
		argn = v
	}

	args := Map{}
	if arging, ok := config["args"].(Map); ok {
		res := ark.Basic.Mapping(arging, value, args, argn, false, ctx)
		if res != nil {
			return nil, res
		}
	}

	service := &Service{
		ctx: ctx, Name: name, Config: config, Setting: setting,
		Value: value, Args: args,
	}

	data := Map{}
	var result *Res

	switch ff := config["action"].(type) {
	case func(*Service):
		ff(service)
	case func(*Service) *Res:
		result = ff(service)

	case func(*Service) bool:
		data = Map{
			"result": ff(service),
		}
	case func(*Service) Map:
		data = ff(service)
	case func(*Service) (Map, *Res):
		data, result = ff(service)
	case func(*Service) []Map:
		items := ff(service)
		data = Map{"items": items}
	case func(*Service) ([]Map, *Res):
		items, res := ff(service)
		data = Map{"items": items}
		result = res
	case func(*Service) (int64, []Map):
		count, items := ff(service)
		data = Map{"count": count, "items": items}
	case func(*Service) (Map, []Map):
		item, items := ff(service)
		data = Map{"item": item, "items": items}
	}

	//参数解析
	if dating, ok := config["data"].(Map); ok {
		out := Map{}
		err := ark.Basic.Mapping(dating, data, out, false, false, ctx)
		if err == nil {
			return out, result
		}
	}

	return data, result
}

func (module *serviceModule) Library(name string) *serviceLibrary {
	return &serviceLibrary{module, name}
}
func (module *serviceModule) Logic(ctx context, name string, settings ...Map) *serviceLogic {
	setting := make(Map)
	if len(settings) > 0 {
		setting = settings[0]
	}
	return &serviceLogic{ctx, name, setting}
}

//------------ library ----------------

func (lib *serviceLibrary) Name() string {
	return lib.name
}
func (lib *serviceLibrary) Method(name string, config Map, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", lib.name, name)
	lib.module.Method(realName, config, overrides...)
}

//------------------- Service 方法 --------------------

func (sv *Service) Result() *Res {
	return sv.ctx.Result()
}
func (lgc *Service) Data(bases ...string) DataBase {
	return lgc.ctx.dataBase(bases...)
}

func (lgc *Service) Invoke(name string, args ...Map) Map {
	var value Map
	if len(args) > 0 {
		value = args[0]
	}
	return lgc.ctx.invoke(name, value)
}

func (lgc *Service) Logic(name string, settings ...Map) *serviceLogic {
	return ark.Service.Logic(lgc.ctx, name, settings...)
}

//------- logic 方法 -------------
func (logic *serviceLogic) naming(name string) string {
	return logic.Name + "." + name
}
func (logic *serviceLogic) Invoke(name string, args ...Map) Map {
	var value Map
	if len(args) > 0 {
		value = args[0]
	}
	return logic.ctx.invoke(logic.naming(name), value, logic.Setting)
}

//-------------------------------------------------------------------------------------------------------

func Method(name string, config Map, overrides ...bool) {
	ark.Service.Method(name, config, overrides...)
}

func Library(name string) *serviceLibrary {
	return ark.Service.Library(name)
}

//触发执行，异步
func Trigger(name string, values ...Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	go ark.Service.Invoke(nil, name, value)
}

//直接执行，同步
func (module *serviceModule) Execute(name string, values ...Map) (Map, *Res) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}

	return ark.Service.Invoke(nil, name, value)
}
