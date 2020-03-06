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
		logic *logicModule

		Name    string
		Setting Map
	}

	Logic struct {
		Name    string
		Config  Map
		Setting Map
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

func (module *logicModule) Library(name string) *logicLibrary {
	return &logicLibrary{module, name}
}
func (module *logicModule) Logic(name string, settings ...Map) *logicLogic {
	setting := make(Map)
	if len(settings) > 0 {
		setting = settings[0]
	}
	return &logicLogic{module, name, setting}
}

func (lib *logicLibrary) Name() string {
	return lib.name
}
func (lib *logicLibrary) Method(name string, config Map, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", lib.name, name)
	lib.module.Method(realName, config, overrides...)
}

func (lgc *Logic) Data(bases ...string) DataBase {
	return nil
}

func (lgc *Logic) Logic(bases ...string) DataBase {
	return nil
}

//-------------------------------------------------------------------------------------------------------

func Method(name string, config Map, overrides ...bool) {
	ark.Logic.Method(name, config, overrides...)
}

func Library(name string) *logicLibrary {
	return ark.Logic.Library(name)
}
