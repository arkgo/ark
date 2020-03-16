package ark

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	. "github.com/arkgo/asset"
	"github.com/arkgo/asset/util"
)

type (
	HttpConfig struct {
		Driver string `toml:"driver"`
		Port   int    `toml:"port"`

		CertFile string `toml:"certfile"`
		KeyFile  string `toml:"keyfile"`

		Charset string `toml:"charset"`
		Domain  string `toml:"domain"`
		Expiry  string `toml:"expiry"`
		MaxAge  string `toml:"maxage"`

		Upload string `toml:"upload"`
		Static string `toml:"static"`
		Shared string `toml:"shared"`

		Setting Map `toml:"setting"`
	}

	HttpDriver interface {
		Connect(HttpConfig) (HttpConnect, error)
	}

	//事件连接
	HttpConnect interface {
		Open() error
		Health() (HttpHealth, error)
		Close() error

		Accept(HttpHandler) error
		Register(name string, config HttpRegister) error

		//开始
		Start() error
		//开始TLS
		StartTLS(certFile, keyFile string) error
	}

	HttpHealth struct {
		Workload int64
	}

	HttpRegister struct {
		Site    string
		Uris    []string
		Methods []string
		Hosts   []string
	}

	HttpHandler func(HttpThread)
	HttpThread  interface {
		Name() string
		Site() string
		Params() Map
		Request() *http.Request
		Response() http.ResponseWriter
		Finish() error
	}

	//跳转
	httpGotoBody struct {
		url string
	}
	httpTextBody struct {
		text string
	}
	httpHtmlBody struct {
		html string
	}
	httpScriptBody struct {
		script string
	}
	httpJsonBody struct {
		json Any
	}
	httpJsonpBody struct {
		json     Any
		callback string
	}
	httpApiBody struct {
		code int
		text string
		data Map
	}
	httpXmlBody struct {
		xml Any
	}
	httpFileBody struct {
		file string
		name string
	}
	httpDownBody struct {
		bytes []byte
		name  string
	}
	httpBufferBody struct {
		buffer io.ReadCloser
		name   string
	}
	httpViewBody struct {
		view  string
		model Any
	}
	httpProxyBody struct {
		url *url.URL
	}

	RawBody string

	httpSite struct {
		module *httpModule
		name   string
		root   string
	}

	httpModule struct {
		mutex   sync.Mutex
		drivers map[string]HttpDriver

		routers       map[string]Router
		routerActions map[string][]HttpFunc

		//拦截器
		requestNames    []string
		requestFilters  map[string]RequestFilter
		requestActions  map[string][]HttpFunc
		executeNames    []string
		executeFilters  map[string]ExecuteFilter
		executeActions  map[string][]HttpFunc
		responseNames   []string
		responseFilters map[string]ResponseFilter
		responseActions map[string][]HttpFunc

		//处理器
		foundNames     []string
		foundHandlers  map[string]FoundHandler
		foundActions   map[string][]HttpFunc
		errorNames     []string
		errorHandlers  map[string]ErrorHandler
		errorActions   map[string][]HttpFunc
		failedNames    []string
		failedHandlers map[string]FailedHandler
		failedActions  map[string][]HttpFunc
		deniedNames    []string
		deniedHandlers map[string]DeniedHandler
		deniedActions  map[string][]HttpFunc

		connect HttpConnect
		url     *httpUrl
	}

	Filter struct {
		site     string   `json:"-"`
		Name     string   `json:"name"`
		Desc     string   `json:"desc"`
		Request  HttpFunc `json:"-"`
		Execute  HttpFunc `json:"-"`
		Response HttpFunc `json:"-"`
	}
	RequestFilter struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}
	ExecuteFilter struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}
	ResponseFilter struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}

	Handler struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Found  HttpFunc `json:"-"`
		Error  HttpFunc `json:"-"`
		Failed HttpFunc `json:"-"`
		Denied HttpFunc `json:"-"`
	}
	FoundHandler struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}
	ErrorHandler struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}
	FailedHandler struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}
	DeniedHandler struct {
		site   string   `json:"-"`
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Action HttpFunc `json:"-"`
	}

	Routing map[string]Router
	Router  struct {
		site     string   `json:"-"`
		Uri      string   `json:"uri"`
		Uris     []string `json:"uris"`
		Name     string   `json:"name"`
		Desc     string   `json:"desc"`
		method   string   `json:"method"` //真实记录实际的method
		Nullable bool     `json:"nullable"`
		Socket   bool     `json:"socket"`
		Setting  Map      `json:"setting"`

		Auth Auth   `json:"auth"`
		Item Item   `json:"item"`
		Args Params `json:"args"`
		Data Params `json:"data"`

		Method  Routing    `json:"-"`
		Action  HttpFunc   `json:"-"`
		Actions []HttpFunc `json:"-"`

		Found  HttpFunc `json:"-"`
		Error  HttpFunc `json:"-"`
		Failed HttpFunc `json:"-"`
		Denied HttpFunc `json:"-"`
	}

	Auth map[string]Sign
	Sign struct {
		Sign    string `json:"sign"`
		Require bool   `json:"require"`
		Base    string `json:"base"`
		Table   string `json:"table"`
		Name    string `json:"name"`
		Desc    string `json:"desc"`
		Empty   *Res   `json:"-"`
		Error   *Res   `json:"-"`
	}
	Item   map[string]Entity
	Entity struct {
		Key     string `json:"key"`
		Require bool   `json:"require"`
		Base    string `json:"base"`
		Table   string `json:"table"`
		Name    string `json:"name"`
		Desc    string `json:"desc"`
		Empty   *Res   `json:"-"`
		Error   *Res   `json:"-"`
	}
)

func newHttp() *httpModule {
	return &httpModule{
		drivers: make(map[string]HttpDriver),

		routers:       make(map[string]Router),
		routerActions: make(map[string][]HttpFunc),

		requestFilters:  make(map[string]RequestFilter),
		requestActions:  make(map[string][]HttpFunc),
		executeFilters:  make(map[string]ExecuteFilter),
		executeActions:  make(map[string][]HttpFunc),
		responseFilters: make(map[string]ResponseFilter),
		responseActions: make(map[string][]HttpFunc),

		foundNames:    make([]string, 0),
		foundHandlers: make(map[string]FoundHandler),
		foundActions:  make(map[string][]HttpFunc),

		errorNames:    make([]string, 0),
		errorHandlers: make(map[string]ErrorHandler),
		errorActions:  make(map[string][]HttpFunc),

		failedNames:    make([]string, 0),
		failedHandlers: make(map[string]FailedHandler),
		failedActions:  make(map[string][]HttpFunc),

		deniedNames:    make([]string, 0),
		deniedHandlers: make(map[string]DeniedHandler),
		deniedActions:  make(map[string][]HttpFunc),

		url: &httpUrl{},
	}

}

//注册HTTP驱动
func (module *httpModule) Driver(name string, driver HttpDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[HTTP]驱动不可为空")
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

func (module *httpModule) connecting(config HttpConfig) (HttpConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(config)
	}
	panic("[HTTP]不支持的驱动" + config.Driver)
}
func (module *httpModule) initing() {

	module.initRouterActions()
	module.initFilterActions()
	module.initHandlerActions()

	connect, err := module.connecting(ark.Config.Http)
	if err != nil {
		panic("[HTTP]连接失败：" + err.Error())
	}
	err = connect.Open()
	if err != nil {
		panic("[HTTP]打开失败：" + err.Error())
	}

	//绑定回调
	connect.Accept(module.serve)

	//注册路由
	for k, v := range module.routers {
		regis := module.registering(v)
		err := connect.Register(k, regis)
		if err != nil {
			panic("[HTTP]注册失败：" + err.Error())
		}
	}

	module.connect = connect
}

func (module *httpModule) registering(config Router) HttpRegister {

	//Uris
	uris := []string{}
	if config.Uri != "" {
		uris = append(uris, config.Uri)
	}
	if config.Uris != nil {
		uris = append(uris, config.Uris...)
	}

	//方法
	methods := []string{}
	if config.method != "" {
		methods = append(methods, config.method)
	}

	site := config.site

	regis := HttpRegister{Site: site, Uris: config.Uris, Methods: methods}

	if cfg, ok := ark.Config.Site[site]; ok {
		regis.Hosts = cfg.Hosts
	}

	return regis
}

func (module *httpModule) Start() {
	if ark.Config.Http.CertFile != "" && ark.Config.Http.KeyFile != "" {
		module.connect.StartTLS(ark.Config.Http.CertFile, ark.Config.Http.KeyFile)
	} else {
		module.connect.Start()
	}
}

func (module *httpModule) exiting() {
	if module.connect != nil {
		module.connect.Close()
	}
}

// func (module *httpModule) Router(name string, config Map, overrides ...bool) {
// 	override := true
// 	if len(overrides) > 0 {
// 		override = overrides[0]
// 	}

// 	names := strings.Split(name, ".")
// 	if len(names) <= 1 {
// 		name = "*." + name
// 	}

// 	//直接的时候直接拆分成目标格式
// 	objects := make(map[string]Map)
// 	if strings.HasPrefix(name, "*.") {
// 		//全站点
// 		for site, _ := range ark.Config.Site {
// 			siteName := strings.Replace(name, "*", site, 1)
// 			siteConfig := make(Map)

// 			//复制配置
// 			for k, v := range config {
// 				siteConfig[k] = v
// 			}
// 			//站点名
// 			siteConfig["site"] = site

// 			//先记录下
// 			objects[siteName] = siteConfig
// 		}
// 	} else {
// 		if len(names) >= 2 {
// 			config["site"] = names[0]
// 		}
// 		//单站点
// 		objects[name] = config
// 	}

// 	//处理对方是单方法，还是多方法
// 	routers := make(map[string]Map)
// 	for routerName, routerConfig := range objects {

// 		if routeConfig, ok := routerConfig["route"].(Map); ok {
// 			//多method版本
// 			for method, vvvv := range routeConfig {
// 				if methodConfig, ok := vvvv.(Map); ok {

// 					realName := fmt.Sprintf("%s.%s", routerName, method)
// 					realConfig := Map{}

// 					//复制全局的定义
// 					for k, v := range routerConfig {
// 						if k != "route" {
// 							realConfig[k] = v
// 						}
// 					}

// 					//复制子级的定义
// 					//注册,args, auth, item等
// 					for k, v := range methodConfig {
// 						if lllMap, ok := v.(Map); ok && (k == "args" || k == "auth" || k == "item") {
// 							if gggMap, ok := realConfig[k].(Map); ok {

// 								newMap := Map{}
// 								//复制全局
// 								for gk, gv := range gggMap {
// 									newMap[gk] = gv
// 								}
// 								//复制方法级
// 								for lk, lv := range lllMap {
// 									newMap[lk] = lv
// 								}

// 								realConfig[k] = newMap

// 							} else {
// 								realConfig[k] = v
// 							}
// 						} else {
// 							realConfig[k] = v
// 						}
// 					}

// 					//相关参数
// 					realConfig["method"] = method

// 					//加入列表
// 					routers[realName] = realConfig
// 				}
// 			}

// 		} else {

// 			//单方法版本
// 			realName := routerName
// 			realConfig := Map{}

// 			//复制定义
// 			for k, v := range routerConfig {
// 				realConfig[k] = v
// 			}

// 			//加入列表
// 			routers[realName] = realConfig
// 		}
// 	}

// 	//这里才是真的注册
// 	for k, v := range routers {
// 		if override {
// 			module.routers[k] = v
// 		} else {
// 			if _, ok := module.routers[k]; ok == false {
// 				module.routers[k] = v
// 			}
// 		}
// 	}
// }

func (module *httpModule) Router(name string, config Router, overrides ...bool) {
	module.mutex.Lock()
	module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	objects := make(map[string]Router)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			siteConfig := config //直接复制一份

			siteConfig.site = site

			//先记录下
			objects[siteName] = siteConfig
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		objects[name] = config
	}

	//处理对方是单方法，还是多方法
	routers := make(map[string]Router)
	for routerName, routerConfig := range objects {

		if routerConfig.Method != nil {
			//多method版本
			for method, methodConfig := range routerConfig.Method {
				realName := fmt.Sprintf("%s.%s", routerName, method)
				realConfig := routerConfig //从顶级复制

				//复制子级的定义
				if methodConfig.Name != "" {
					realConfig.Name = methodConfig.Name
				}
				if methodConfig.Desc != "" {
					realConfig.Desc = methodConfig.Desc
				}

				//复制设置
				if methodConfig.Setting != nil {
					if realConfig.Setting == nil {
						realConfig.Setting = make(Map)
					}
					for k, v := range methodConfig.Setting {
						realConfig.Setting[k] = v
					}
				}

				//复制args
				if methodConfig.Args != nil {
					if realConfig.Args == nil {
						realConfig.Args = Params{}
					}
					for k, v := range methodConfig.Args {
						realConfig.Args[k] = v
					}
				}

				//复制data
				if methodConfig.Data != nil {
					if realConfig.Data == nil {
						realConfig.Data = Params{}
					}
					for k, v := range methodConfig.Data {
						realConfig.Data[k] = v
					}
				}
				//复制auth
				//待优化：是否使用专用类型
				if methodConfig.Auth != nil {
					if realConfig.Auth == nil {
						realConfig.Auth = Auth{}
					}
					for k, v := range methodConfig.Auth {
						realConfig.Auth[k] = v
					}
				}
				//复制item
				//待优化：是否使用专用类型
				if methodConfig.Item != nil {
					if realConfig.Item == nil {
						realConfig.Item = Item{}
					}
					for k, v := range methodConfig.Item {
						realConfig.Item[k] = v
					}
				}

				//复制方法
				if methodConfig.Action != nil {
					realConfig.Action = methodConfig.Action
				}
				if methodConfig.Actions != nil {
					realConfig.Actions = methodConfig.Actions
				}

				//复制处理器
				if methodConfig.Found != nil {
					realConfig.Found = methodConfig.Found
				}
				if methodConfig.Error != nil {
					realConfig.Error = methodConfig.Error
				}
				if methodConfig.Failed != nil {
					realConfig.Failed = methodConfig.Failed
				}
				if methodConfig.Denied != nil {
					realConfig.Denied = methodConfig.Denied
				}

				//相关参数
				realConfig.method = method

				//加入列表
				routers[realName] = realConfig
			}

		} else {
			//单方法版本
			routers[routerName] = routerConfig
		}
	}

	//这里才是真的注册
	for key, val := range routers {
		//一些默认的处理

		//复制uri到uris，默认使用uris
		if val.Uris == nil {
			val.Uris = make([]string, 1)
			val.Uris = append(val.Uris, val.Uri)
			val.Uri = ""
		}
		//复制action
		if val.Actions == nil {
			val.Actions = make([]HttpFunc, 1)
			val.Actions = append(val.Actions, val.Action)
			val.Action = nil
		}

		//这里全局置空
		val.Method = nil

		if override {
			module.routers[key] = val
		} else {
			if _, ok := module.routers[key]; ok == false {
				module.routers[key] = val
			}
		}
	}
}

func (module *httpModule) initRouterActions() {
	for name, config := range module.routers {
		if _, ok := module.routerActions[name]; ok == false {
			module.routerActions[name] = make([]HttpFunc, 0)
		}

		if config.Action != nil {
			module.routerActions[name] = append(module.routerActions[name], config.Action)
		}
		if config.Actions != nil {
			module.routerActions[name] = append(module.routerActions[name], config.Actions...)
		}
	}
}
func (module *httpModule) Routers(sites ...string) map[string]Router {
	prefix := ""
	if len(sites) > 0 {
		prefix = sites[0] + "."
	}

	routers := make(map[string]Router)
	for name, config := range module.routers {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			routers[name] = config
		}
	}

	return routers
}
func (module *httpModule) Filter(name string, config Filter, overrides ...bool) {
	if config.Request != nil {
		module.RequestFilter(name, RequestFilter{config.site, config.Name, config.Desc, config.Request}, overrides...)
	}
	if config.Execute != nil {
		module.ExecuteFilter(name, ExecuteFilter{config.site, config.Name, config.Desc, config.Execute}, overrides...)
	}
	if config.Response != nil {
		module.ResponseFilter(name, ResponseFilter{config.site, config.Name, config.Desc, config.Response}, overrides...)
	}
}

func (module *httpModule) RequestFilter(name string, config RequestFilter, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	filters := make(map[string]RequestFilter)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			filters[siteName] = RequestFilter{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		filters[name] = config
	}

	for key, val := range filters {
		if override {
			module.requestFilters[key] = val
			module.requestNames = append(module.requestNames, key)
		} else {
			if _, ok := module.requestFilters[key]; ok == false {
				module.requestFilters[key] = val
				module.requestNames = append(module.requestNames, key)
			}
		}
	}
}

func (module *httpModule) ExecuteFilter(name string, config ExecuteFilter, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	filters := make(map[string]ExecuteFilter)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			filters[siteName] = ExecuteFilter{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		filters[name] = config
	}

	for key, val := range filters {
		if override {
			module.executeFilters[key] = val
			module.executeNames = append(module.executeNames, key)
		} else {
			if _, ok := module.executeFilters[key]; ok == false {
				module.executeFilters[key] = val
				module.executeNames = append(module.executeNames, key)
			}
		}
	}
}

func (module *httpModule) ResponseFilter(name string, config ResponseFilter, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	filters := make(map[string]ResponseFilter)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			filters[siteName] = ResponseFilter{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		filters[name] = config
	}

	for key, val := range filters {
		if override {
			module.responseFilters[key] = val
			module.responseNames = append(module.responseNames, key)
		} else {
			if _, ok := module.responseFilters[key]; ok == false {
				module.responseFilters[key] = val
				module.responseNames = append(module.responseNames, key)
			}
		}
	}
}

func (module *httpModule) initFilterActions() {
	//请求拦截器
	for _, name := range module.requestNames {
		config := module.requestFilters[name]
		if _, ok := module.requestActions[config.site]; ok == false {
			module.requestActions[config.site] = make([]HttpFunc, 0)
		}
		module.requestActions[config.site] = append(module.requestActions[config.site], config.Action)
	}

	//执行拦截器
	for _, name := range module.executeNames {
		config := module.executeFilters[name]
		if _, ok := module.executeActions[config.site]; ok == false {
			module.executeActions[config.site] = make([]HttpFunc, 0)
		}
		module.executeActions[config.site] = append(module.executeActions[config.site], config.Action)
	}

	//响应拦截器
	for _, name := range module.responseNames {
		config := module.responseFilters[name]
		if _, ok := module.responseActions[config.site]; ok == false {
			module.responseActions[config.site] = make([]HttpFunc, 0)
		}
		module.responseActions[config.site] = append(module.responseActions[config.site], config.Action)
	}
}

func (module *httpModule) Handler(name string, config Handler, overrides ...bool) {
	if config.Found != nil {
		module.FoundHandler(name, FoundHandler{config.site, config.Name, config.Desc, config.Found}, overrides...)
	}
	if config.Error != nil {
		module.ErrorHandler(name, ErrorHandler{config.site, config.Name, config.Desc, config.Error}, overrides...)
	}
	if config.Failed != nil {
		module.FailedHandler(name, FailedHandler{config.site, config.Name, config.Desc, config.Failed}, overrides...)
	}
	if config.Denied != nil {
		module.DeniedHandler(name, DeniedHandler{config.site, config.Name, config.Desc, config.Denied}, overrides...)
	}
}

func (module *httpModule) FoundHandler(name string, config FoundHandler, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	handlers := make(map[string]FoundHandler)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			handlers[siteName] = FoundHandler{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		handlers[name] = config
	}

	for key, val := range handlers {
		if override {
			module.foundHandlers[key] = val
			module.foundNames = append(module.foundNames, key)
		} else {
			if _, ok := module.foundHandlers[key]; ok == false {
				module.foundHandlers[key] = val
				module.foundNames = append(module.foundNames, key)
			}
		}
	}
}

func (module *httpModule) ErrorHandler(name string, config ErrorHandler, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	handlers := make(map[string]ErrorHandler)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			handlers[siteName] = ErrorHandler{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		handlers[name] = config
	}

	for key, val := range handlers {
		if override {
			module.errorHandlers[key] = val
			module.errorNames = append(module.errorNames, key)
		} else {
			if _, ok := module.errorHandlers[key]; ok == false {
				module.errorHandlers[key] = val
				module.errorNames = append(module.errorNames, key)
			}
		}
	}
}

func (module *httpModule) FailedHandler(name string, config FailedHandler, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	handlers := make(map[string]FailedHandler)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			handlers[siteName] = FailedHandler{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		handlers[name] = config
	}

	for key, val := range handlers {
		if override {
			module.failedHandlers[key] = val
			module.failedNames = append(module.failedNames, key)
		} else {
			if _, ok := module.failedHandlers[key]; ok == false {
				module.failedHandlers[key] = val
				module.failedNames = append(module.failedNames, key)
			}
		}
	}
}

func (module *httpModule) DeniedHandler(name string, config DeniedHandler, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	//从名称里找站点
	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	handlers := make(map[string]DeniedHandler)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			handlers[siteName] = DeniedHandler{
				site, config.Name, config.Desc, config.Action,
			}
		}
	} else {
		if len(names) >= 2 {
			config.site = names[0]
		}
		//单站点
		handlers[name] = config
	}

	for key, val := range handlers {
		if override {
			module.deniedHandlers[key] = val
			module.deniedNames = append(module.deniedNames, key)
		} else {
			if _, ok := module.deniedHandlers[key]; ok == false {
				module.deniedHandlers[key] = val
				module.deniedNames = append(module.deniedNames, key)
			}
		}
	}
}

func (module *httpModule) initHandlerActions() {
	//found处理器
	for _, name := range module.foundNames {
		config := module.foundHandlers[name]
		if _, ok := module.foundActions[config.site]; ok == false {
			module.foundActions[config.site] = make([]HttpFunc, 0)
		}
		module.foundActions[config.site] = append(module.foundActions[config.site], config.Action)
	}

	//error处理器
	for _, name := range module.errorNames {
		config := module.errorHandlers[name]
		if _, ok := module.errorActions[config.site]; ok == false {
			module.errorActions[config.site] = make([]HttpFunc, 0)
		}
		module.errorActions[config.site] = append(module.errorActions[config.site], config.Action)
	}

	//failed处理器
	for _, name := range module.failedNames {
		config := module.failedHandlers[name]
		if _, ok := module.failedActions[config.site]; ok == false {
			module.failedActions[config.site] = make([]HttpFunc, 0)
		}
		module.failedActions[config.site] = append(module.failedActions[config.site], config.Action)
	}

	//denied处理器
	for _, name := range module.deniedNames {
		config := module.deniedHandlers[name]
		if _, ok := module.deniedActions[config.site]; ok == false {
			module.deniedActions[config.site] = make([]HttpFunc, 0)
		}
		module.deniedActions[config.site] = append(module.deniedActions[config.site], config.Action)
	}
}

//事件Http  请求开始
func (module *httpModule) serve(thread HttpThread) {

	ctx := httpContext(thread)
	if config, ok := module.routers[ctx.Name]; ok {
		ctx.Config = config
		if config.Setting != nil {
			ctx.Setting = config.Setting
		}
	}

	//request拦截器，加入调用列表
	if funcs, ok := module.requestActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}
	ctx.next(module.request)
	ctx.next(module.execute)

	//开始执行
	ctx.Next()
}

func (module *httpModule) request(ctx *Http) {
	now := time.Now()

	//请求id
	ctx.Id = ctx.Cookie(ctx.siteConfig.Cookie)
	if ctx.Id == "" {
		ctx.Id = ark.Codec.Unique()
		ctx.Cookie(ctx.siteConfig.Cookie, ctx.Id)
		ctx.Session("$last", now.Unix())
	} else {
		//请求的一开始，主要是SESSION处理
		if ctx.sessional(true) {
			mmm, eee := ark.Session.Read(ctx.Id)
			if eee == nil && mmm != nil {
				for k, v := range mmm {
					ctx.sessions[k] = v
				}
			} else {
				//活跃超过1天，就更新一下session
				if vv, ok := ctx.sessions["$last"].(float64); ok {
					if (now.Unix() - int64(vv)) > 60*60*24 {
						ctx.Session("$last", now.Unix())
					}
				} else {
					ctx.Session("$last", now.Unix())
				}
			}
		}
	}

	//404么
	if ctx.Name == "" {

		//路由不存在， 找静态文件

		//静态文件放在这里处理
		file := ""
		sitePath := path.Join(ark.Config.Http.Static, ctx.Site, ctx.Path)
		if fi, err := os.Stat(sitePath); err == nil && fi.IsDir() == false {
			file = sitePath
		} else {
			sharedPath := path.Join(ark.Config.Http.Static, ark.Config.Http.Shared, ctx.Path)
			if fi, err := os.Stat(sharedPath); err == nil && fi.IsDir() == false {
				file = sharedPath
			}
		}

		if file != "" {
			ctx.File(file, "")
		} else {
			ctx.Found()
		}

	} else {
		//表单这里处理，这样会在 requestFilter之前处理好
		if res := ctx.formHandler(); res.Fail() {
			ctx.lastError = res
			module.failed(ctx)
		} else {
			if res := ctx.clientHandler(); res.Fail() {
				ctx.lastError = res
				module.failed(ctx)
			} else {
				if err := ctx.argsHandler(); err != nil {
					ctx.lastError = err
					module.failed(ctx)
				} else {
					if err := ctx.authHandler(); err != nil {
						ctx.lastError = err
						module.denied(ctx)
					} else {
						if err := ctx.itemHandler(); err != nil {
							ctx.lastError = err
							module.failed(ctx)
						} else {
							//往下走，到execute
							ctx.Next()
						}
					}
				}
			}
		}
	}

	//session写回去
	if ctx.sessional(false) {
		//这样节省SESSION的资源
		if ctx.siteConfig.Expiry != "" {
			td, err := util.ParseDuration(ctx.siteConfig.Expiry)
			if err == nil {
				ark.Session.Write(ctx.Id, ctx.sessions, td)
			} else {
				ark.Session.Write(ctx.Id, ctx.sessions)
			}
		} else {
			ark.Session.Write(ctx.Id, ctx.sessions)
		}
	}

	module.response(ctx)
}

//事件执行，调用action的地方
func (module *httpModule) execute(ctx *Http) {
	ctx.clear()

	//executeFilters
	if funcs, ok := module.executeActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	// //actions
	if funcs, ok := module.routerActions[ctx.Name]; ok {
		ctx.next(funcs...)
	}
	// funcs := ctx.funcing("action")
	// ctx.next(funcs...)

	ctx.Next()
}

//事件执行，调用action的地方
func (module *httpModule) response(ctx *Http) {
	//响应前清空执行线
	ctx.clear()

	//response拦截器，加入调用列表
	if funcs, ok := module.responseActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//最终的body处理，加入执行线
	ctx.next(module.body)

	ctx.Next()
}

//最终响应
func (module *httpModule) body(ctx *Http) {
	if ctx.Code == 0 {
		ctx.Code = http.StatusOK
	}

	//设置cookies, headers

	//cookie超时时间
	//为了极致的性能，可以在启动的时候先解析好
	var maxage time.Duration
	if ctx.siteConfig.MaxAge != "" {
		td, err := util.ParseDuration(ctx.siteConfig.MaxAge)
		if err == nil {
			maxage = td
		}
	}

	for _, v := range ctx.cookies {

		if ctx.domain != "" {
			v.Domain = ctx.domain
		}
		if ctx.siteConfig.MaxAge != "" {
			v.MaxAge = int(maxage.Seconds())
		}

		http.SetCookie(ctx.response, &v)
	}
	for k, v := range ctx.headers {
		ctx.response.Header().Set(k, v)
	}

	switch body := ctx.Body.(type) {
	case httpGotoBody:
		module.bodyGoto(ctx, body)
	case httpTextBody:
		module.bodyText(ctx, body)
	case httpHtmlBody:
		module.bodyHtml(ctx, body)
	case httpScriptBody:
		module.bodyScript(ctx, body)
	case httpJsonBody:
		module.bodyJson(ctx, body)
	case httpJsonpBody:
		module.bodyJsonp(ctx, body)
	case httpApiBody:
		module.bodyApi(ctx, body)
	case httpXmlBody:
		module.bodyXml(ctx, body)
	case httpFileBody:
		module.bodyFile(ctx, body)
	case httpDownBody:
		module.bodyDown(ctx, body)
	case httpBufferBody:
		module.bodyBuffer(ctx, body)
	case httpViewBody:
		module.bodyView(ctx, body)
	case httpProxyBody:
		module.bodyProxy(ctx, body)
	default:
		module.bodyDefault(ctx)
	}

	//最终响应前做清理工作
	ctx.terminal()
}
func (module *httpModule) bodyDefault(ctx *Http) {
	ctx.Code = http.StatusNotFound
	http.NotFound(ctx.response, ctx.request)
	ctx.thread.Finish()
}
func (module *httpModule) bodyGoto(ctx *Http, body httpGotoBody) {
	http.Redirect(ctx.response, ctx.request, body.url, http.StatusFound)
	ctx.thread.Finish()
}
func (module *httpModule) bodyText(ctx *Http, body httpTextBody) {
	res := ctx.response

	if ctx.Type == "" {
		ctx.Type = "text"
	}

	ctx.Type = ark.Basic.Mimetype(ctx.Type, "text/explain")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

	res.WriteHeader(ctx.Code)
	fmt.Fprint(res, body.text)

	ctx.thread.Finish()
}
func (module *httpModule) bodyHtml(ctx *Http, body httpHtmlBody) {
	res := ctx.response

	if ctx.Type == "" {
		ctx.Type = "html"
	}

	ctx.Type = ark.Basic.Mimetype(ctx.Type, "text/html")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

	res.WriteHeader(ctx.Code)
	fmt.Fprint(res, body.html)

	ctx.thread.Finish()
}
func (module *httpModule) bodyScript(ctx *Http, body httpScriptBody) {
	res := ctx.response

	ctx.Type = ark.Basic.Mimetype(ctx.Type, "application/script")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

	res.WriteHeader(ctx.Code)
	fmt.Fprint(res, body.script)
	ctx.thread.Finish()
}
func (module *httpModule) bodyJson(ctx *Http, body httpJsonBody) {
	res := ctx.response

	bytes, err := ark.Codec.Marshal(body.json)
	if err != nil {
		//要不要发到统一的错误ctx.Error那里？再走一遍
		http.Error(res, err.Error(), http.StatusInternalServerError)
	} else {

		ctx.Type = ark.Basic.Mimetype(ctx.Type, "text/json")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, string(bytes))
	}
	ctx.thread.Finish()
}
func (module *httpModule) bodyJsonp(ctx *Http, body httpJsonpBody) {
	res := ctx.response

	bytes, err := ark.Codec.Marshal(body.json)
	if err != nil {
		//要不要发到统一的错误ctx.Error那里？再走一遍
		http.Error(res, err.Error(), http.StatusInternalServerError)
	} else {

		ctx.Type = ark.Basic.Mimetype(ctx.Type, "application/script")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, fmt.Sprintf("%s(%s);", body.callback, string(bytes)))
	}
	ctx.thread.Finish()
}
func (module *httpModule) bodyXml(ctx *Http, body httpXmlBody) {
	res := ctx.response

	if ctx.Type == "" {
		ctx.Type = "xml"
	}

	content := ""
	if vv, ok := body.xml.(string); ok {
		content = vv
	} else {
		bytes, err := xml.Marshal(body.xml)
		if err == nil {
			content = string(bytes)
		}
	}

	if content == "" {
		http.Error(res, "解析xml失败", http.StatusInternalServerError)
	} else {
		ctx.Type = ark.Basic.Mimetype(ctx.Type, "text/xml")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, content)
	}
	ctx.thread.Finish()
}
func (module *httpModule) bodyApi(ctx *Http, body httpApiBody) {

	json := Map{
		"code": body.code,
		"time": time.Now().Unix(),
	}

	if body.text != "" {
		json["text"] = body.text
	}

	if body.data != nil {

		crypto := ctx.siteConfig.Crypto
		if vv, ok := ctx.Setting["crypto"].(bool); ok && vv == false {
			crypto = ""
		}
		if vv, ok := ctx.Setting["plain"].(bool); ok && vv {
			crypto = ""
		}

		tempConfig := Params{
			"data": Param{
				Type: "json", Require: true, Encode: crypto,
			},
		}
		tempData := Map{
			"data": body.data,
		}

		//有自定义返回数据类型
		if ctx.Config.Data != nil {
			tempConfig = Params{
				"data": Param{
					Type: "json", Require: true, Encode: crypto,
					Children: ctx.Config.Data,
				},
			}
		}

		val := Map{}
		res := ark.Basic.Mapping(tempConfig, tempData, val, false, false, ctx)

		//Debug("json", tempConfig, tempData)

		if res.OK() {
			//处理后的data
			json["data"] = val["data"]
		} else {
			json["code"] = ark.Basic.Code(res.Text)
			json["text"] = ctx.String(res.Text, res.Args...)
		}

	}

	//转到jsonbody去处理
	module.bodyJson(ctx, httpJsonBody{json})
}

func (module *httpModule) bodyFile(ctx *Http, body httpFileBody) {
	req, res := ctx.request, ctx.response

	//文件类型
	if ctx.Type != "file" {
		ctx.Type = ark.Basic.Mimetype(ctx.Type, "application/octet-stream")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))
	}
	//加入自定义文件名
	if body.name != "" {
		res.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%v;", body.name))
	}

	http.ServeFile(res, req, body.file)
	ctx.thread.Finish()
}
func (module *httpModule) bodyDown(ctx *Http, body httpDownBody) {
	res := ctx.response

	if ctx.Type == "" {
		ctx.Type = "file"
	}

	ctx.Type = ark.Basic.Mimetype(ctx.Type, "application/octet-stream")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))
	//加入自定义文件名
	if body.name != "" {
		res.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%v;", body.name))
	}

	res.WriteHeader(ctx.Code)
	res.Write(body.bytes)

	ctx.thread.Finish()
}
func (module *httpModule) bodyBuffer(ctx *Http, body httpBufferBody) {
	res := ctx.response

	if ctx.Type == "" {
		ctx.Type = "file"
	}

	ctx.Type = ark.Basic.Mimetype(ctx.Type, "application/octet-stream")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))
	//加入自定义文件名
	if body.name != "" {
		res.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%v;", body.name))
	}

	res.WriteHeader(ctx.Code)
	_, err := io.Copy(res, body.buffer)
	//bytes,err := ioutil.ReadAll(body.buffer)
	if err == nil {
		http.Error(res, "read buffer error", http.StatusInternalServerError)
	}
	body.buffer.Close()
	ctx.thread.Finish()
}
func (module *httpModule) bodyView(ctx *Http, body httpViewBody) {
	res := ctx.response

	viewdata := Map{
		"args": ctx.Args, "auth": ctx.Auth,
		"stting": Setting, "local": ctx.Local,
		"data": ctx.Data, "model": body.model,
	}

	helpers := module.viewHelpers(ctx)

	html, err := ark.View.Parse(ViewBody{
		Helpers: helpers,
		Root:    ark.Config.View.Root,
		Shared:  ark.Config.Http.Shared,
		View:    body.view, Data: viewdata,
		Site: ctx.Site, Lang: ctx.Lang(), Zone: ctx.Zone(),
	})

	if err != nil {
		http.Error(res, ctx.String(err.Error()), 500)
	} else {
		mime := ark.Basic.Mimetype(ctx.Type, "text/html")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", mime, ctx.Charset))
		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, html)
	}

	ctx.thread.Finish()
}
func (module *httpModule) viewHelpers(ctx *Http) Map {
	//系统内置的helper
	helpers := Map{
		"route":    ctx.Url.Route,
		"browse":   ctx.Url.Browse,
		"preview":  ctx.Url.Preview,
		"download": ctx.Url.Download,
		"backurl":  ctx.Url.Back,
		"lasturl":  ctx.Url.Last,
		"siteurl": func(name string, paths ...string) string {
			path := "/"
			if len(paths) > 0 {
				path = paths[0]
			}
			return ctx.Url.Site(name, path)
		},

		"lang": func() string {
			return ctx.Lang()
		},
		"zone": func() *time.Location {
			return ctx.Zone()
		},
		"timezone": func() string {
			return ctx.String(ctx.Zone().String())
		},
		"format": func(format string, args ...interface{}) string {
			//支持一下显示时间
			if len(args) == 1 {
				if args[0] == nil {
					return format
				} else if ttt, ok := args[0].(time.Time); ok {
					zoneTime := ttt.In(ctx.Zone())
					return zoneTime.Format(format)
				} else if ttt, ok := args[0].(int64); ok {
					//时间戳是大于1971年是, 千万级, 2016年就是10亿级了
					if ttt >= int64(31507200) && ttt <= int64(31507200000) {
						ttt := time.Unix(ttt, 0)
						zoneTime := ttt.In(ctx.Zone())
						sss := zoneTime.Format(format)
						if strings.HasPrefix(sss, "%") == false || format != sss {
							return sss
						}
					}
				}
			}
			return fmt.Sprintf(format, args...)
		},

		"signed": func(key string) bool {
			return ctx.Signed(key)
		},
		"signal": func(key string) string {
			return ctx.Signal(key)
		},
		"signer": func(key string) string {
			return ctx.Signer(key)
		},
		"string": func(key string, args ...Any) string {
			return ctx.String(key, args...)
		},
		"option": func(name, field string, v Any) string {
			value := fmt.Sprintf("%v", v)
			//多语言支持
			//key=enum.name.file.value
			langkey := fmt.Sprintf("option_%s_%s_%s", name, field, value)
			langval := ctx.String(langkey)
			if langkey != langval {
				return langval
			} else {
				return ark.Data.Option(name, field, value)
				// if vv, ok := enums[value].(string); ok {
				// 	return vv
				// }
				// return value
			}
		},
	}

	for k, v := range ark.View.actions {
		if f, ok := v.(func(*Http, ...Any) Any); ok {
			helpers[k] = func(args ...Any) Any {
				return f(ctx, args...)
			}
		} else {
			helpers[k] = v
		}
	}

	return helpers
}

func (module *httpModule) bodyProxy(ctx *Http, body httpProxyBody) {
	req := ctx.request
	res := ctx.response

	target := body.url
	targetQuery := body.url.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}

	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(res, req)

	ctx.thread.Finish()
}

// func (module *httpModule) sessionKey(ctx *Http) string {
// 	format := "http_%s"
// 	if vv,ok := CONFIG.Session.Format[bHTTP].(string); ok && vv != "" {
// 		format = vv
// 	}
// 	return fmt.Sprintf(format, ctx.Id)
// }

//事件handler,找不到
func (module *httpModule) found(ctx *Http) {
	ctx.clear()

	if ctx.Code <= 0 {
		ctx.Code = http.StatusNotFound
	}

	//如果有自定义的错误处理，加入调用列表
	// funcs := ctx.funcing("found")
	if ctx.Config.Found != nil {
		ctx.next(ctx.Config.Found)
	}

	//把处理器加入调用列表
	if funcs, ok := module.foundActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.foundDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) foundDefaultHandler(ctx *Http) {
	found := newResult("_found")
	if res := ctx.Result(); res != nil {
		found = res
	}

	ctx.Code = http.StatusNotFound

	//如果是ajax访问，返回JSON对应，要不然返回页面
	if ctx.Ajax {
		ctx.Answer(found)
	} else {
		ctx.Text("http not found")
	}
}

//事件handler,错误的处理
func (module *httpModule) error(ctx *Http) {
	ctx.clear()

	if ctx.Code <= 0 {
		ctx.Code = http.StatusInternalServerError
	}

	//如果有自定义的错误处理，加入调用列表
	// funcs := ctx.funcing("error")
	if ctx.Config.Error != nil {
		ctx.next(ctx.Config.Error)
	}

	//把错误处理器加入调用列表
	if funcs, ok := module.errorActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.errorDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) errorDefaultHandler(ctx *Http) {
	error := newResult("_error")
	if res := ctx.Result(); res != nil {
		error = res
	}

	ctx.Code = http.StatusInternalServerError

	if ctx.Ajax {
		ctx.Answer(error)
	} else {

		ctx.Data["status"] = ctx.Code
		ctx.Data["error"] = Map{
			"code": ark.Basic.Code(error.Text),
			"text": ctx.String(error.Text, error.Args...),
		}

		ctx.View("error")
	}
}

//事件handler,失败处理，主要是args失败
func (module *httpModule) failed(ctx *Http) {
	ctx.clear()

	if ctx.Code <= 0 {
		ctx.Code = http.StatusBadRequest
	}

	//如果有自定义的失败处理，加入调用列表
	// funcs := ctx.funcing("failed")
	// ctx.next(funcs...)
	if ctx.Config.Failed != nil {
		ctx.next(ctx.Config.Failed)
	}

	//把失败处理器加入调用列表
	if funcs, ok := module.failedActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.failedDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) failedDefaultHandler(ctx *Http) {
	failed := newResult("_failed")
	if res := ctx.Result(); res != nil {
		failed = res
	}

	ctx.Code = http.StatusBadRequest

	if ctx.Ajax {
		ctx.Answer(failed)
	} else {
		ctx.Alert(failed)
	}
}

//事件handler,失败处理，主要是args失败
func (module *httpModule) denied(ctx *Http) {
	ctx.clear()

	if ctx.Code <= 0 {
		ctx.Code = http.StatusUnauthorized
	}

	//如果有自定义的失败处理，加入调用列表
	// funcs := ctx.funcing("denied")
	// ctx.next(funcs...)
	if ctx.Config.Denied != nil {
		ctx.next(ctx.Config.Denied)
	}

	//把失败处理器加入调用列表
	if funcs, ok := module.deniedActions[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.deniedDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
//如果是ajax。返回拒绝
//如果不是， 返回一个脚本提示
func (module *httpModule) deniedDefaultHandler(ctx *Http) {
	denied := newResult("_denied")
	if res := ctx.Result(); res != nil {
		denied = res
	}

	ctx.Code = http.StatusUnauthorized

	if ctx.Ajax {
		ctx.Answer(denied)
	} else {
		ctx.Alert(denied)
	}
}

func (module *httpModule) newSite(name string, roots ...string) *httpSite {
	root := ""
	if len(roots) > 0 {
		root = strings.TrimRight(roots[0], "/")
	}
	return &httpSite{module, name, root}
}

func Site(name string, roots ...string) *httpSite {
	return ark.Http.newSite(name, roots...)
}
func Route(name string, args ...Map) string {
	return ark.Http.url.Route(name, args...)
}
func Routers(sites ...string) map[string]Router {
	return ark.Http.Routers(sites...)
}
