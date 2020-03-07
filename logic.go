package ark

import (
	"fmt"
	"sync"

	. "github.com/arkgo/base"
)

type (
	logicModule struct {
		mutex   sync.Mutex
		methods map[string]Map
	}

	logicLibrary struct {
		module *logicModule
		name   string
	}

	logicLogic struct {
		ctx     context
		Name    string
		Setting Map
	}

	Logic struct {
		ctx     context
		Name    string
		Config  Map
		Setting Map
		Value   Map
		Args    Map
	}
)

func newLogic() *logicModule {
	return &logicModule{
		methods: make(map[string]Map, 0),
	}
}

//注册方法
func (module *logicModule) Method(name string, config Map, overrides ...bool) {
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

func (module *logicModule) Invoke(name string, value Map, setting Map, ctxs ...context) (Map, *Res) {
	var config Map
	if vv, ok := module.methods[name]; ok {
		config = vv
	}

	if config == nil {
		return nil, Fail
	}

	if value == nil {
		value = Map{}
	}
	if setting == nil {
		setting = Map{}
	}
	var ctx context
	if len(ctxs) > 0 {
		ctx = ctxs[0]
	}
	if ctx == nil {
		ctx = emptyContext()
		defer ctx.terminal()
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

	logic := &Logic{
		ctx: ctx, Name: name, Config: config, Setting: setting,
		Value: value, Args: args,
	}

	data := Map{}
	var result *Res

	switch ff := config["action"].(type) {
	case func(*Logic):
		ff(logic)
	case func(*Logic) *Res:
		result = ff(logic)

	case func(*Logic) bool:
		data = Map{
			"result": ff(logic),
		}
	case func(*Logic) Map:
		data = ff(logic)
	case func(*Logic) (Map, *Res):
		data, result = ff(logic)
	case func(*Logic) []Map:
		items := ff(logic)
		data = Map{"items": items}
	case func(*Logic) ([]Map, *Res):
		items, res := ff(logic)
		data = Map{"items": items}
		result = res
	case func(*Logic) (int64, []Map):
		count, items := ff(logic)
		data = Map{"count": count, "items": items}
	case func(*Logic) (Map, []Map):
		item, items := ff(logic)
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

func (module *logicModule) Library(name string) *logicLibrary {
	return &logicLibrary{module, name}
}
func (module *logicModule) Logic(ctx context, name string, settings ...Map) *logicLogic {
	setting := make(Map)
	if len(settings) > 0 {
		setting = settings[0]
	}
	return &logicLogic{ctx, name, setting}
}

func (lib *logicLibrary) Name() string {
	return lib.name
}
func (lib *logicLibrary) Method(name string, config Map, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", lib.name, name)
	lib.module.Method(realName, config, overrides...)
}

//------------------- Logic 方法 --------------------

func (sv *Logic) Result() *Res {
	return sv.ctx.Result()
}
func (lgc *Logic) Data(bases ...string) DataBase {
	return lgc.ctx.dataBase(bases...)
}

func (lgc *Logic) Invoke(name string, args ...Map) Map {
	return lgc.ctx.Invoke(name, args...)
}

func (lgc *Logic) Logic(name string, settings ...Map) *logicLogic {
	return ark.Logic.Logic(lgc.ctx, name, settings...)
}

//-------------------------------------------------------------------------------------------------------

func Method(name string, config Map, overrides ...bool) {
	ark.Logic.Method(name, config, overrides...)
}

func Library(name string) *logicLibrary {
	return ark.Logic.Library(name)
}
