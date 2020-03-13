package ark

import (
	"errors"
	"sync"
	"time"

	. "github.com/arkgo/base"
)

type (
	ViewConfig struct {
		Driver  string `toml:"driver"`
		Root    string `toml:"root"`
		Shared  string `toml:"shared"`
		Left    string `toml:"left"`
		Right   string `toml:"right"`
		Setting Map    `toml:"setting"`
	}
	//视图驱动
	ViewDriver interface {
		Connect(config ViewConfig) (ViewConnect, error)
	}
	//视图连接
	ViewConnect interface {
		//打开、关闭
		Open() error
		Health() (ViewHealth, error)
		Close() error

		Parse(ViewBody) (string, error)
	}

	ViewHealth struct {
		Workload int64
	}

	ViewBody struct {
		Root    string
		Shared  string
		View    string
		Site    string
		Lang    string
		Zone    *time.Location
		Data    Map
		Helpers Map
	}

	viewModule struct {
		mutex   sync.Mutex
		drivers map[string]ViewDriver
		helpers map[string]Helper
		actions Map

		//视图配置，视图连接
		connect ViewConnect
	}

	Helper struct {
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Alias  []string `json:"alias"`
		Action Any      `json:"-"`
	}
)

func newView() *viewModule {
	return &viewModule{
		drivers: make(map[string]ViewDriver),
		helpers: make(map[string]Helper),
		actions: make(Map),
	}
}

func (module *viewModule) Driver(name string, driver ViewDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[视图]]驱动不可为空")
	}

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	if override {
		module.drivers[name] = driver
	} else {
		if module.drivers[name] == nil {
			module.drivers[name] = driver
		}
	}
}

// func (module *viewModule) Helper(name string, helper Map, overrides ...bool) {
// 	module.mutex.Lock()
// 	defer module.mutex.Unlock()

// 	if helper == nil {
// 		panic("[视图]方法不可为空")
// 	}

// 	override := true
// 	if len(overrides) > 0 {
// 		override = overrides[0]
// 	}

// 	if override {
// 		module.helper(name, helper)
// 	} else {
// 		if module.helpers[name] == nil {
// 			module.helper(name, helper)
// 		}
// 	}
// }

func (module *viewModule) Helper(name string, config Helper, overrides ...bool) {
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
			module.helpers[key] = config
			module.actions[key] = config.Action
		} else {
			if _, ok := module.helpers[key]; ok == false {
				module.helpers[key] = config
				module.actions[key] = config.Action
			}
		}

	}
}

// func (module *viewModule) helper(name string, helper Map) {
// 	module.helpers[name] = helper
// 	if action, ok := helper["action"]; ok {
// 		module.actions[name] = action
// 	}
// }
func (module *viewModule) connecting(config ViewConfig) (ViewConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(config)
	}
	panic("[视图]不支持的驱动：" + config.Driver)
}

//初始化
func (module *viewModule) initing() {

	//连接视图
	connect, err := module.connecting(ark.Config.View)
	if err != nil {
		panic("[视图]连接失败：" + err.Error())
	}

	//打开连接
	err = connect.Open()
	if err != nil {
		panic("[视图]打开失败：" + err.Error())
	}

	//保存连接
	module.connect = connect
}

//退出
func (module *viewModule) exiting() {
	if module.connect != nil {
		module.connect.Close()
	}
}

func (module *viewModule) Parse(body ViewBody) (string, error) {
	if module.connect == nil {
		return "", errors.New("[会话]无效连接")
	}
	return module.connect.Parse(body)
}

// func Helper(name string, config Map, overrides ...bool) {
// 	ark.View.Helper(name, config, overrides...)
// }
