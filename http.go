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

	"github.com/arkgo/asset/util"
	. "github.com/arkgo/base"
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

	httpSiteFuncs = map[string][]HttpFunc
	httpModule    struct {
		mutex   sync.Mutex
		drivers map[string]HttpDriver

		routers       map[string]Map
		routerActions httpSiteFuncs

		//为了提高效率，这里直接保存
		filters         map[string]Map
		requestFilters  httpSiteFuncs
		executeFilters  httpSiteFuncs
		responseFilters httpSiteFuncs

		handlers       map[string]Map
		foundHandlers  httpSiteFuncs
		errorHandlers  httpSiteFuncs
		failedHandlers httpSiteFuncs
		deniedHandlers httpSiteFuncs

		connect HttpConnect
		url     *httpUrl
	}
)

func newHttp() *httpModule {
	return &httpModule{
		drivers: make(map[string]HttpDriver),

		routers:       make(map[string]Map),
		routerActions: make(httpSiteFuncs),

		filters:         make(map[string]Map),
		requestFilters:  make(httpSiteFuncs),
		executeFilters:  make(httpSiteFuncs),
		responseFilters: make(httpSiteFuncs),

		handlers:       make(map[string]Map),
		foundHandlers:  make(httpSiteFuncs),
		errorHandlers:  make(httpSiteFuncs),
		failedHandlers: make(httpSiteFuncs),
		deniedHandlers: make(httpSiteFuncs),

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

func (module *httpModule) registering(config Map) HttpRegister {

	//Uris
	uris := []string{}
	if vv, ok := config["uri"].(string); ok && vv != "" {
		uris = append(uris, vv)
	}
	if vv, ok := config["uris"].([]string); ok {
		uris = append(uris, vv...)
	}
	//方法
	methods := []string{}
	if vv, ok := config["method"].(string); ok && vv != "" {
		methods = append(methods, vv)
	}
	if vv, ok := config["methods"].([]string); ok {
		methods = append(methods, vv...)
	}

	site := ""
	if vv, ok := config["site"].(string); ok {
		site = vv
	}

	regis := HttpRegister{Site: site, Uris: uris, Methods: methods}

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

func (module *httpModule) Router(name string, config Map, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	objects := make(map[string]Map)
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			siteConfig := make(Map)

			//复制配置
			for k, v := range config {
				siteConfig[k] = v
			}
			//站点名
			siteConfig["site"] = site

			//先记录下
			objects[siteName] = siteConfig
		}
	} else {
		if len(names) >= 2 {
			config["site"] = names[0]
		}
		//单站点
		objects[name] = config
	}

	//处理对方是单方法，还是多方法
	routers := make(map[string]Map)
	for routerName, routerConfig := range objects {

		if routeConfig, ok := routerConfig["route"].(Map); ok {
			//多method版本
			for method, vvvv := range routeConfig {
				if methodConfig, ok := vvvv.(Map); ok {

					realName := fmt.Sprintf("%s.%s", routerName, method)
					realConfig := Map{}

					//复制全局的定义
					for k, v := range routerConfig {
						if k != "route" {
							realConfig[k] = v
						}
					}

					//复制子级的定义
					//注册,args, auth, item等
					for k, v := range methodConfig {
						if lllMap, ok := v.(Map); ok && (k == "args" || k == "auth" || k == "item") {
							if gggMap, ok := realConfig[k].(Map); ok {

								newMap := Map{}
								//复制全局
								for gk, gv := range gggMap {
									newMap[gk] = gv
								}
								//复制方法级
								for lk, lv := range lllMap {
									newMap[lk] = lv
								}

								realConfig[k] = newMap

							} else {
								realConfig[k] = v
							}
						} else {
							realConfig[k] = v
						}
					}

					//相关参数
					realConfig["method"] = method

					//加入列表
					routers[realName] = realConfig
				}
			}

		} else {

			//单方法版本
			realName := routerName
			realConfig := Map{}

			//复制定义
			for k, v := range routerConfig {
				realConfig[k] = v
			}

			//加入列表
			routers[realName] = realConfig
		}
	}

	//这里才是真的注册
	for k, v := range routers {
		if override {
			module.routers[k] = v
		} else {
			if _, ok := module.routers[k]; ok == false {
				module.routers[k] = v
			}
		}
	}
}
func (module *httpModule) initRouterActions() {
	for name, config := range module.routers {
		if _, ok := module.routerActions[name]; ok == false {
			module.routerActions[name] = make([]HttpFunc, 0)
		}
		switch v := config["action"].(type) {
		case func(*Http):
			module.routerActions[name] = append(module.routerActions[name], v)
		case []func(*Http):
			for _, vv := range v {
				module.routerActions[name] = append(module.routerActions[name], vv)
			}
		case HttpFunc:
			module.routerActions[name] = append(module.routerActions[name], v)
		case []HttpFunc:
			module.routerActions[name] = append(module.routerActions[name], v...)
		}
	}
}
func (module *httpModule) Routers(sites ...string) map[string]Map {
	prefix := ""
	if len(sites) > 0 {
		prefix = sites[0] + "."
	}

	routers := make(map[string]Map)
	for name, config := range module.routers {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			mmm := Map{} //为了配置安全，复制数据
			for k, v := range config {
				mmm[k] = v
			}
			routers[name] = mmm
		}
	}

	return routers
}
func (module *httpModule) Filter(name string, config Map, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	filters := map[string]Map{}
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			siteConfig := Map{}

			//复制配置
			for k, v := range config {
				siteConfig[k] = v
			}
			//站点名
			siteConfig["site"] = site

			//先记录下
			filters[siteName] = siteConfig
		}
	} else {
		if len(names) >= 2 {
			config["site"] = names[0]
		}
		//单站点
		filters[name] = config
	}

	//这里才是真的注册
	for k, v := range filters {
		if override {
			module.filters[k] = v
		} else {
			if _, ok := module.filters[k]; ok == false {
				module.filters[k] = v
			}
		}
	}
}
func (module *httpModule) initFilterActions() {

	for _, config := range module.filters {
		site := ""
		if vv, ok := config["site"].(string); ok {
			site = vv
		}

		//请求拦截器
		if _, ok := module.requestFilters[site]; ok == false {
			module.requestFilters[site] = make([]HttpFunc, 0)
		}
		switch v := config["request"].(type) {
		case func(*Http):
			module.requestFilters[site] = append(module.requestFilters[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.requestFilters[site] = append(module.requestFilters[site], vv)
			}
		case HttpFunc:
			module.requestFilters[site] = append(module.requestFilters[site], v)
		case []HttpFunc:
			module.requestFilters[site] = append(module.requestFilters[site], v...)
		}

		//执行拦截器
		if _, ok := module.executeFilters[site]; ok == false {
			module.executeFilters[site] = make([]HttpFunc, 0)
		}
		switch v := config["execute"].(type) {
		case func(*Http):
			module.executeFilters[site] = append(module.executeFilters[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.executeFilters[site] = append(module.executeFilters[site], vv)
			}
		case HttpFunc:
			module.executeFilters[site] = append(module.executeFilters[site], v)
		case []HttpFunc:
			module.executeFilters[site] = append(module.executeFilters[site], v...)
		}

		//响应拦截器
		if _, ok := module.responseFilters[site]; ok == false {
			module.responseFilters[site] = make([]HttpFunc, 0)
		}
		switch v := config["response"].(type) {
		case func(*Http):
			module.responseFilters[site] = append(module.responseFilters[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.responseFilters[site] = append(module.responseFilters[site], vv)
			}
		case HttpFunc:
			module.responseFilters[site] = append(module.responseFilters[site], v)
		case []HttpFunc:
			module.responseFilters[site] = append(module.responseFilters[site], v...)
		}
	}
}
func (module *httpModule) Handler(name string, config Map, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	names := strings.Split(name, ".")
	if len(names) <= 1 {
		name = "*." + name
	}

	//直接的时候直接拆分成目标格式
	handlers := map[string]Map{}
	if strings.HasPrefix(name, "*.") {
		//全站点
		for site, _ := range ark.Config.Site {
			siteName := strings.Replace(name, "*", site, 1)
			siteConfig := Map{}

			//复制配置
			for k, v := range config {
				siteConfig[k] = v
			}
			//站点名
			siteConfig["site"] = site

			//先记录下
			handlers[siteName] = siteConfig
		}
	} else {
		if len(names) >= 2 {
			config["site"] = names[0]
		}
		//单站点
		handlers[name] = config
	}

	//这里才是真的注册
	for k, v := range handlers {
		if override {
			module.handlers[k] = v
		} else {
			if _, ok := module.handlers[k]; ok == false {
				module.handlers[k] = v
			}
		}
	}
}
func (module *httpModule) initHandlerActions() {

	for _, config := range module.handlers {
		site := ""
		if vv, ok := config["site"].(string); ok {
			site = vv
		}

		//不存在处理器
		if _, ok := module.foundHandlers[site]; ok == false {
			module.foundHandlers[site] = make([]HttpFunc, 0)
		}
		switch v := config["found"].(type) {
		case func(*Http):
			module.foundHandlers[site] = append(module.foundHandlers[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.foundHandlers[site] = append(module.foundHandlers[site], vv)
			}
		case HttpFunc:
			module.foundHandlers[site] = append(module.foundHandlers[site], v)
		case []HttpFunc:
			module.foundHandlers[site] = append(module.foundHandlers[site], v...)
		}

		//错误处理器
		if _, ok := module.errorHandlers[site]; ok == false {
			module.errorHandlers[site] = make([]HttpFunc, 0)
		}
		switch v := config["error"].(type) {
		case func(*Http):
			module.errorHandlers[site] = append(module.errorHandlers[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.errorHandlers[site] = append(module.errorHandlers[site], vv)
			}
		case HttpFunc:
			module.errorHandlers[site] = append(module.errorHandlers[site], v)
		case []HttpFunc:
			module.errorHandlers[site] = append(module.errorHandlers[site], v...)
		}

		//失败处理器
		if _, ok := module.failedHandlers[site]; ok == false {
			module.failedHandlers[site] = make([]HttpFunc, 0)
		}
		switch v := config["failed"].(type) {
		case func(*Http):
			module.failedHandlers[site] = append(module.failedHandlers[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.failedHandlers[site] = append(module.failedHandlers[site], vv)
			}
		case HttpFunc:
			module.failedHandlers[site] = append(module.failedHandlers[site], v)
		case []HttpFunc:
			module.failedHandlers[site] = append(module.failedHandlers[site], v...)
		}

		//拒绝处理器
		if _, ok := module.deniedHandlers[site]; ok == false {
			module.deniedHandlers[site] = make([]HttpFunc, 0)
		}
		switch v := config["denied"].(type) {
		case func(*Http):
			module.deniedHandlers[site] = append(module.deniedHandlers[site], v)
		case []func(*Http):
			for _, vv := range v {
				module.deniedHandlers[site] = append(module.deniedHandlers[site], vv)
			}
		case HttpFunc:
			module.deniedHandlers[site] = append(module.deniedHandlers[site], v)
		case []HttpFunc:
			module.deniedHandlers[site] = append(module.deniedHandlers[site], v...)
		}
	}

}

//事件Http  请求开始
func (module *httpModule) serve(thread HttpThread) {

	ctx := httpContext(thread)
	if config, ok := module.routers[ctx.Name]; ok {
		ctx.Config = config
	}

	//Debug("ccc", ctx.Name, ctx.Config)

	//request拦截器，加入调用列表
	if funcs, ok := module.requestFilters[ctx.Site]; ok {
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
		ctx.Id = Unique()
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
	if ctx.Name == "" || ctx.Config == nil {

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
	if funcs, ok := module.executeFilters[ctx.Site]; ok {
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
	if funcs, ok := module.responseFilters[ctx.Site]; ok {
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
		if vv, ok := ctx.Config["crypto"].(bool); ok && vv == false {
			crypto = ""
		}
		if vv, ok := ctx.Config["plain"].(bool); ok && vv {
			crypto = ""
		}

		tempConfig := Map{
			"data": Map{
				"type": "json", "must": true, "encode": crypto,
			},
		}
		tempData := Map{
			"data": body.data,
		}

		//有自定义返回数据类型
		if vv, ok := ctx.Config["data"].(Map); ok {
			tempConfig = Map{
				"data": Map{
					"type": "json", "must": true, "encode": crypto,
					"json": vv,
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
		"enum": func(name, field string, v Any) string {
			value := fmt.Sprintf("%v", v)
			//多语言支持
			//key=enum.name.file.value
			langkey := fmt.Sprintf("enum.%s.%s.%s", name, field, value)
			langval := ctx.String(langkey)
			if langkey != langval {
				return langval
			} else {
				enums := Enums(name, field)
				if vv, ok := enums[value].(string); ok {
					return vv
				}
				return value
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

	ctx.Code = http.StatusNotFound

	//如果有自定义的错误处理，加入调用列表
	funcs := ctx.funcing("found")
	ctx.next(funcs...)

	//把处理器加入调用列表
	if funcs, ok := module.foundHandlers[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.foundDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) foundDefaultHandler(ctx *Http) {
	found := newResult("found")
	if res := ctx.Result(); res != nil {
		found = res
	}

	//如果是ajax访问，返回JSON对应，要不然返回页面
	if ctx.Ajax {
		ctx.Answer(found)
	} else {
		ctx.View("found")
	}
}

//事件handler,错误的处理
func (module *httpModule) error(ctx *Http) {
	ctx.clear()

	//如果有自定义的错误处理，加入调用列表
	funcs := ctx.funcing("error")
	ctx.next(funcs...)

	//把错误处理器加入调用列表
	if funcs, ok := module.errorHandlers[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.errorDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) errorDefaultHandler(ctx *Http) {
	error := newResult("error")
	if res := ctx.Result(); res != nil {
		error = res
	}

	ctx.Code = http.StatusInternalServerError
	if error == Found {
		ctx.Code = 404
	}

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

	//如果有自定义的失败处理，加入调用列表
	funcs := ctx.funcing("failed")
	ctx.next(funcs...)

	//把失败处理器加入调用列表
	if funcs, ok := module.failedHandlers[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//加入默认的错误处理
	ctx.next(module.failedDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) failedDefaultHandler(ctx *Http) {
	failed := newResult("failed")
	if res := ctx.Result(); res != nil {
		failed = res
	}

	if ctx.Ajax {
		ctx.Answer(failed)
	} else {
		ctx.Alert(failed)
	}
}

//事件handler,失败处理，主要是args失败
func (module *httpModule) denied(ctx *Http) {
	ctx.clear()

	//如果有自定义的失败处理，加入调用列表
	funcs := ctx.funcing("denied")
	ctx.next(funcs...)

	//把失败处理器加入调用列表
	if funcs, ok := module.deniedHandlers[ctx.Site]; ok {
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
	denied := newResult("denied")
	if res := ctx.Result(); res != nil {
		denied = res
	}

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

func (site *httpSite) Route(name string, args ...Map) string {
	realName := fmt.Sprintf("%s.%s", site.name, name)
	return ark.Http.url.Route(realName, args...)
}
func (site *httpSite) Router(name string, config Map, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", site.name, name)
	if site.root != "" {
		if uri, ok := config["uri"].(string); ok {
			config["uri"] = site.root + uri
		} else if uris, ok := config["uris"].([]string); ok {
			for i, uri := range uris {
				uris[i] = site.root + uri
			}
			config["uris"] = uris
		}
	}
	site.module.Router(realName, config, overrides...)
}
func (site *httpSite) Filter(name string, config Map, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", site.name, name)
	site.module.Filter(realName, config, overrides...)
}
func (site *httpSite) RequestFilter(name string, config Map) {
	config["request"] = config["action"]
	delete(config, "action")
	site.Filter(name, config)
}
func (site *httpSite) ExecuteFilter(name string, config Map) {
	config["execute"] = config["action"]
	delete(config, "action")
	site.Filter(name, config)
}
func (site *httpSite) ResponseFilter(name string, config Map) {
	config["response"] = config["action"]
	delete(config, "action")
	site.Filter(name, config)
}
func (site *httpSite) Handler(name string, config Map, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", site.name, name)
	site.module.Handler(realName, config, overrides...)
}
func (site *httpSite) FoundHandler(name string, config Map) {
	config["found"] = config["action"]
	delete(config, "action")
	site.Handler(name, config)
}
func (site *httpSite) ErrorHandler(name string, config Map) {
	config["error"] = config["action"]
	delete(config, "action")
	site.Handler(name, config)
}
func (site *httpSite) FailedHandler(name string, config Map) {
	config["failed"] = config["action"]
	delete(config, "action")
	site.Handler(name, config)
}
func (site *httpSite) DeniedHandler(name string, config Map) {
	config["denied"] = config["action"]
	delete(config, "action")
	site.Handler(name, config)
}

// func Filter(name string, config Map, overrides ...bool) {
// 	ark.Http.Filter(name, config, overrides...)
// }
// func Handler(name string, config Map, overrides ...bool) {
// 	ark.Http.Handler(name, config, overrides...)
// }
// func Router(name string, config Map, overrides ...bool) {
// 	ark.Http.Router(name, config, overrides...)
// }

func Site(name string, roots ...string) *httpSite {
	return ark.Http.newSite(name, roots...)
}
func Route(name string, args ...Map) string {
	return ark.Http.url.Route(name, args...)
}
func Routers(sites ...string) map[string]Map {
	return ark.Http.Routers(sites...)
}
