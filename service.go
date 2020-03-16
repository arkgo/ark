package ark

import (
	"fmt"
	"sync"
	"time"

	. "github.com/arkgo/asset"
)

type (
	serviceModule struct {
		mutex   sync.Mutex
		methods map[string]Method
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
		Config  Method
		Setting Map
		Value   Map
		Args    Map
	}

	Method struct {
		Name     string   `json:"name"`
		Desc     string   `json:"desc"`
		Alias    []string `json:"alias"`
		Nullable bool     `json:"nullable"`
		Args     Params   `json:"args"`
		Data     Params   `json:"data"`
		Setting  Map      `json:"setting"`
		Action   Any      `json:"-"`
	}
)

func newService() *serviceModule {
	return &serviceModule{
		methods: make(map[string]Method, 0),
	}
}

//注册方法
func (module *serviceModule) Method(name string, config Method, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

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

	for _, key := range alias {
		if override {
			module.methods[key] = config
		} else {
			if _, ok := module.methods[key]; ok == false {
				module.methods[key] = config
			}
		}
	}
}

func (module *serviceModule) Invoke(ctx context, name string, value Map, settings ...Map) (Map, *Res) {
	if _, ok := module.methods[name]; ok == false {
		return nil, Fail
	}

	config := module.methods[name]

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

	args := Map{}
	if config.Args != nil {
		res := ark.Basic.Mapping(config.Args, value, args, config.Nullable, false, ctx)
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

	switch ff := config.Action.(type) {
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
	if config.Data != nil {
		out := Map{}
		err := ark.Basic.Mapping(config.Data, data, out, false, false, ctx)
		if err == nil {
			return out, result
		}
	}

	//参数如果解析失败，就原版返回
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
func (lib *serviceLibrary) Register(name string, config Method, overrides ...bool) {
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

//语法糖
func (lgc *Service) Locked(key string, expiry time.Duration, cons ...string) bool {
	return ark.Mutex.Lock(key, expiry, cons...) != nil
}
func (lgc *Service) Lock(key string, expiry time.Duration, cons ...string) error {
	return ark.Mutex.Lock(key, expiry, cons...)
}
func (lgc *Service) Unlock(key string, cons ...string) error {
	return ark.Mutex.Unlock(key, cons...)
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

// func Method(name string, config Map, overrides ...bool) {
// 	ark.Service.Method(name, config, overrides...)
// }

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
