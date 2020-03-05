package ark

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/arkgo/asset/hashring"
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

	httpRegisters = map[string]Map
	httpSiteFuncs = map[string][]HttpFunc
	httpModule    struct {
		drivers map[string]HttpDriver

		routers       httpRegisters
		routerActions httpSiteFuncs

		//为了提高效率，这里直接保存
		filters         httpRegisters
		requestFilters  httpSiteFuncs
		executeFilters  httpSiteFuncs
		responseFilters httpSiteFuncs

		handlers      httpRegisters
		errorFilters  httpSiteFuncs
		failedFilters httpSiteFuncs
		deniedFilters httpSiteFuncs

		connect HttpConnect
	}
)

func newHttp() *httpModule {
	return &httpModule{
		drivers: make(map[string]HttpDriver),

		routers:       make(httpRegisters),
		routerActions: make(httpSiteFuncs),

		filters:         make(httpRegisters),
		requestFilters:  make(httpSiteFuncs),
		executeFilters:  make(httpSiteFuncs),
		responseFilters: make(httpSiteFuncs),

		handlers:      make(httpRegisters),
		errorFilters:  make(httpSiteFuncs),
		failedFilters: make(httpSiteFuncs),
		deniedFilters: make(httpSiteFuncs),
	}
}

//注册缓存驱动
func (module *httpModule) Driver(name string, driver CacheDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[缓存]驱动不可为空")
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

func (module *httpModule) connecting(name string, config CacheConfig) (CacheConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(name, config)
	}
	panic("[缓存]不支持的驱动" + config.Driver)
}
func (module *httpModule) initing() {
	weights := make(map[string]int)
	for name, config := range ark.Config.Cache {
		if config.Weight > 0 {
			//只有设置了权重的缓存才参与分布
			weights[name] = config.Weight
		}

		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[缓存]连接失败：" + err.Error())
		}
		err = connect.Open()
		if err != nil {
			panic("[缓存]打开失败：" + err.Error())
		}

		//绑定回调
		connect.Accept(module.serve)

		//注册路由
		for k, v := range module.routers {
			regis := module.registering(vv)
			err := connect.Register(k, regis)
			if err != nil {
				panic("[HTTP]注册失败：" + err.Error())
			}
		}

		module.connects[name] = connect
	}

	//hashring分片
	module.weights = weights
	module.hashring = hashring.New(weights)
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

func (module *httpModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

func (module *httpModule) Router(name string, config Map, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
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
func (module *httpModule) router(name string, config Map) {
	module.routers[name] = config

	if _, ok := module.routerActions[name]; ok == false {
		module.routerActions[name] = make([]HttpFunc, 0)
	}
	switch v := config["action"].(type) {
	case func(*Http):
		module.routerActions = append(module.routerActions, v)
	case []func(*Http):
		for _, vv := range v {
			module.routerActions = append(module.routerActions, vv)
		}
	case HttpFunc:
		module.routerActions = append(module.routerActions, v)
	case []HttpFunc:
		module.routerActions = append(module.routerActions, v...)
	}

}
func (module *httpModule) Filter(name string, config Map, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
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
		//单站点
		filters[name] = config
	}

	//这里才是真的注册
	for k, v := range filters {
		if override {
			module.filter(k, v)
		} else {
			if _, ok := module.filters[k]; ok == false {
				module.filter(k, v)
			}
		}
	}
}
func (module *httpModule) filter(name string, config Map) {
	module.filters[name] = config

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
		module.requestFilters = append(module.requestFilters, v)
	case []func(*Http):
		for _, vv := range v {
			module.requestFilters = append(module.requestFilters, vv)
		}
	case HttpFunc:
		module.requestFilters = append(module.requestFilters, v)
	case []HttpFunc:
		module.requestFilters = append(module.requestFilters, v...)
	}

	//执行拦截器
	if _, ok := module.executeFilters[site]; ok == false {
		module.executeFilters[site] = make([]HttpFunc, 0)
	}
	switch v := config["execute"].(type) {
	case func(*Http):
		module.executeFilters = append(module.executeFilters, v)
	case []func(*Http):
		for _, vv := range v {
			module.executeFilters = append(module.executeFilters, vv)
		}
	case HttpFunc:
		module.executeFilters = append(module.executeFilters, v)
	case []HttpFunc:
		module.executeFilters = append(module.executeFilters, v...)
	}

	//响应拦截器
	if _, ok := module.responseFilters[site]; ok == false {
		module.responseFilters[site] = make([]HttpFunc, 0)
	}
	switch v := config["response"].(type) {
	case func(*Http):
		module.responseFilters = append(module.responseFilters, v)
	case []func(*Http):
		for _, vv := range v {
			module.responseFilters = append(module.responseFilters, vv)
		}
	case HttpFunc:
		module.responseFilters = append(module.responseFilters, v)
	case []HttpFunc:
		module.responseFilters = append(module.responseFilters, v...)
	}
}
func (module *httpModule) Handler(name string, config Map, overrides ...bool) {
	override := true
	if len(overrides) > 0 {
		override = overrides[0]
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
		//单站点
		handlers[name] = config
	}

	//这里才是真的注册
	for k, v := range handlers {
		if override {
			// module.handler[k] = v
			module.handler(k, v)
		} else {
			if _, ok := module.handler[k]; ok == false {
				// module.handler[k] = v
				module.handler(k, v)
			}
		}
	}
}
func (module *httpModule) handler(name string, config Map) {
	module.handlers[name] = config

	site := ""
	if vv, ok := config["site"].(string); ok {
		site = vv
	}

	//错误处理器
	if _, ok := module.errorHandlers[site]; ok == false {
		module.errorHandlers[site] = make([]HttpFunc, 0)
	}
	switch v := config["error"].(type) {
	case func(*Http):
		module.errorHandlers = append(module.errorHandlers, v)
	case []func(*Http):
		for _, vv := range v {
			module.errorHandlers = append(module.errorHandlers, vv)
		}
	case HttpFunc:
		module.errorHandlers = append(module.errorHandlers, v)
	case []HttpFunc:
		module.errorHandlers = append(module.errorHandlers, v...)
	}

	//失败处理器
	if _, ok := module.failedHandlers[site]; ok == false {
		module.failedHandlers[site] = make([]HttpFunc, 0)
	}
	switch v := config["failed"].(type) {
	case func(*Http):
		module.failedHandlers = append(module.failedHandlers, v)
	case []func(*Http):
		for _, vv := range v {
			module.failedHandlers = append(module.failedHandlers, vv)
		}
	case HttpFunc:
		module.failedHandlers = append(module.failedHandlers, v)
	case []HttpFunc:
		module.failedHandlers = append(module.failedHandlers, v...)
	}

	//拒绝处理器
	if _, ok := module.deniedHandlers[site]; ok == false {
		module.deniedHandlers[site] = make([]HttpFunc, 0)
	}
	switch v := config["denied"].(type) {
	case func(*Http):
		module.deniedHandlers = append(module.deniedHandlers, v)
	case []func(*Http):
		for _, vv := range v {
			module.deniedHandlers = append(module.deniedHandlers, vv)
		}
	case HttpFunc:
		module.deniedHandlers = append(module.deniedHandlers, v)
	case []HttpFunc:
		module.deniedHandlers = append(module.deniedHandlers, v...)
	}

}

//事件Http  请求开始
func (module *httpModule) serve(thread HttpThread) {
	ctx := httpContext(thread)
	if config, ok := module.routers[ctx.Name].(Map); ok {
		ctx.Config = config
	}

	//Debug("ccc", ctx.Name, ctx.Config)

	//request拦截器，加入调用列表
	if funcs, ok := module.requestFilters[ctx.Site]; ok {
		ctx.next(funcs...)
	}
	ctx.next(module.request)

	//开始执行
	ctx.Next()
}

func (module *httpModule) request(ctx *Http) {

	//请求id
	ctx.Id = ctx.Cookie(ctx.siteConfig.Cookie)
	if ctx.Id == "" {
		ctx.Id = Unique()
		ctx.Cookie(ctx.siteConfig.Cookie, ctx.Id)
	}

	//请求的一开始，主要是SESSION处理
	if ctx.sessional(true) {
		mmm, eee := ark.Session.Read(ctx.Id)
		if eee == nil {
			for k, v := range mmm {
				//待处理，session要不要写入value，好让args可以处理
				ctx.Session[k] = v
			}
		}
	}

	//404么
	if ctx.Name == "" || ctx.Config == nil {

		//路由不存在， 找静态文件

		//静态文件放在这里处理
		file := ""
		sitePath := path.Join(ark.Config.Http.Static, ctx.Site, ctx.Uri)
		if fi, err := os.Stat(sitePath); err == nil && fi.IsDir() == false {
			file = sitePath
		} else {
			sharedPath := path.Join(ark.Config.Http.Static, ark.Config.Http.Shared, ctx.Uri)
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
			if res := module.clientHandler(ctx); res.Fail() {
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
							ctx.execute()
						}
					}
				}
			}
		}
	}

	//session写回去
	if ctx.sessional(true) {
		//待处理，SESSION如果没有任何变化， 就不写session
		//这样节省SESSION的资源
		if ctx.siteConfig.Expiry != "" {
			td, err := util.ParseDuration(ctx.siteConfig.Expiry)
			if err == nil {
				ark.Session.Write(ctx.Id, ctx.Session, td)
			} else {
				ark.Session.Write(ctx.Id, ctx.Session)
			}
		} else {
			ark.Session.Write(ctx.Id, ctx.Session)
		}
	}

	ctx.response()
}

//事件执行，调用action的地方
func (module *httpModule) execute(ctx *Http) {
	ctx.clear()

	//executeFilters
	if funcs, ok := module.executeFilters[ctx.Site]; ok {
		ctx.next(funcs...)
	}

	//actions
	if funcs, ok := module.routerActions[ctx.Name]; ok {
		ctx.next(funcs...)
	}

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
		res.Header().Set(k, v)
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
	ctx.final()
}
func (module *httpModule) bodyDefault(ctx *Http) {
	ctx.Code = http.StatusNotFound
	http.NotFound(ctx.req.Writer, ctx.req.Reader)
	ctx.thread.Finish()
}
func (module *httpModule) bodyGoto(ctx *Http, body httpGotoBody) {
	http.Redirect(ctx.req.Writer, ctx.req.Reader, body.url, http.StatusFound)
	ctx.thread.Finish()
}
func (module *httpModule) bodyText(ctx *Http, body httpTextBody) {
	res := ctx.req.Writer

	if ctx.Type == "" {
		ctx.Type = "text"
	}

	ctx.Type = mBase.Mimetype(ctx.Type, "text/explain")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

	res.WriteHeader(ctx.Code)
	fmt.Fprint(res, body.text)

	ctx.thread.Finish()
}
func (module *httpModule) bodyHtml(ctx *Http, body httpHtmlBody) {
	res := ctx.req.Writer

	if ctx.Type == "" {
		ctx.Type = "html"
	}

	ctx.Type = mBase.Mimetype(ctx.Type, "text/html")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

	res.WriteHeader(ctx.Code)
	fmt.Fprint(res, body.html)

	ctx.thread.Finish()
}
func (module *httpModule) bodyScript(ctx *Http, body httpScriptBody) {
	res := ctx.req.Writer

	ctx.Type = mBase.Mimetype(ctx.Type, "application/script")
	res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

	res.WriteHeader(ctx.Code)
	fmt.Fprint(res, body.script)
	ctx.thread.Finish()
}
func (module *httpModule) bodyJson(ctx *Http, body httpJsonBody) {
	res := ctx.req.Writer

	bytes, err := json.Marshal(body.json)
	if err != nil {
		//要不要发到统一的错误ctx.Error那里？再走一遍
		http.Error(res, err.Error(), http.StatusInternalServerError)
	} else {

		ctx.Type = mBase.Mimetype(ctx.Type, "text/json")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, string(bytes))
	}
	ctx.thread.Finish()
}
func (module *httpModule) bodyJsonp(ctx *Http, body httpJsonpBody) {
	res := ctx.req.Writer

	bytes, err := json.Marshal(body.json)
	if err != nil {
		//要不要发到统一的错误ctx.Error那里？再走一遍
		http.Error(res, err.Error(), http.StatusInternalServerError)
	} else {

		ctx.Type = mBase.Mimetype(ctx.Type, "application/script")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", ctx.Type, ctx.Charset))

		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, fmt.Sprintf("%s(%s);", body.callback, string(bytes)))
	}
	ctx.thread.Finish()
}
func (module *httpModule) bodyXml(ctx *Http, body httpXmlBody) {
	res := ctx.req.Writer

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
		ctx.Type = mBase.Mimetype(ctx.Type, "text/xml")
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
		res := mBase.Mapping(tempConfig, tempData, val, false, false, &ctx.Context)

		//Debug("json", tempConfig, tempData)

		if res.OK() {
			//处理后的data
			json["data"] = val["data"]
		} else {
			json["code"] = mBase.Code(res.Text)
			json["text"] = ctx.String(res.Text, res.Args...)
		}

	}

	//转到jsonbody去处理
	module.bodyJson(ctx, httpJsonBody{json})
}

func (module *httpModule) bodyFile(ctx *Http, body httpFileBody) {
	req, res := ctx.req.Reader, ctx.req.Writer

	//文件类型
	if ctx.Type != "file" {
		ctx.Type = mBase.Mimetype(ctx.Type, "application/octet-stream")
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
	res := ctx.req.Writer

	if ctx.Type == "" {
		ctx.Type = "file"
	}

	ctx.Type = mBase.Mimetype(ctx.Type, "application/octet-stream")
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
	res := ctx.req.Writer

	if ctx.Type == "" {
		ctx.Type = "file"
	}

	ctx.Type = mBase.Mimetype(ctx.Type, "application/octet-stream")
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
	res := ctx.req.Writer

	viewdata := Map{
		kARGS: ctx.Args, kAUTH: ctx.Auth,
		kSETTING: Setting, kLOCAL: ctx.Local,
		kDATA: ctx.Data, kMODEL: body.model,
	}

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
			return ctx.Lang
		},
		"zone": func() *time.Location {
			return ctx.Zone
		},
		"timezone": func() string {
			return ctx.String(ctx.Zone.String())
		},
		"format": func(format string, args ...interface{}) string {
			//支持一下显示时间
			if len(args) == 1 {
				if args[0] == nil {
					return format
				} else if ttt, ok := args[0].(time.Time); ok {
					zoneTime := ttt.In(ctx.Zone)
					return zoneTime.Format(format)
				} else if ttt, ok := args[0].(int64); ok {
					//时间戳是大于1971年是, 千万级, 2016年就是10亿级了
					if ttt >= int64(31507200) && ttt <= int64(31507200000) {
						ttt := time.Unix(ttt, 0)
						zoneTime := ttt.In(ctx.Zone)
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

	vhelpers := mView.helperActions()
	for k, v := range vhelpers {
		helpers[k] = v
	}

	html, err := mView.parse(ctx, ViewBody{
		Root:    Config.Path.View,
		Shared:  Config.Path.Shared,
		View:    body.view,
		Data:    viewdata,
		Helpers: helpers,
	})

	if err != nil {
		http.Error(res, ctx.String(err.Error()), 500)
	} else {
		mime := mBase.Mimetype(ctx.Type, "text/html")
		res.Header().Set("Content-Type", fmt.Sprintf("%v; charset=%v", mime, ctx.Charset))
		res.WriteHeader(ctx.Code)
		fmt.Fprint(res, html)
	}

	ctx.thread.Finish()
}

func (module *httpModule) bodyProxy(ctx *Http, body httpProxyBody) {
	req := ctx.req.Reader
	res := ctx.req.Writer

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
	funcs := ctx.funcing(kFOUND)
	ctx.next(funcs...)

	//把处理器加入调用列表
	handlers := module.foundHandlerActions(ctx.Site)
	ctx.next(handlers...)

	//加入默认的错误处理
	ctx.next(module.foundDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) foundDefaultHandler(ctx *Http) {
	found := newResult(_kFOUND)
	if res := ctx.Result(); res != nil {
		found = res
	}

	//如果是ajax访问，返回JSON对应，要不然返回页面
	if ctx.Ajax {
		ctx.Answer(found)
	} else {
		ctx.View(kFOUND)
	}
}

//事件handler,错误的处理
func (module *httpModule) error(ctx *Http) {
	ctx.clear()

	//如果有自定义的错误处理，加入调用列表
	funcs := ctx.funcing(kERROR)
	ctx.next(funcs...)

	//把错误处理器加入调用列表
	handlers := module.errorHandlerActions(ctx.Site)
	ctx.next(handlers...)

	//加入默认的错误处理
	ctx.next(module.errorDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) errorDefaultHandler(ctx *Http) {
	error := newResult(_kERROR)
	if res := ctx.Result(); res != nil {
		error = res
	}
	if ctx.Ajax {
		ctx.Answer(error)
	} else {
		ctx.Data[kERROR] = Map{
			"code": mBase.Code(error.Text),
			"text": ctx.String(error.Text, error.Args...),
		}
		ctx.View(kERROR)
	}
}

//事件handler,失败处理，主要是args失败
func (module *httpModule) failed(ctx *Http) {
	ctx.clear()

	//如果有自定义的失败处理，加入调用列表
	funcs := ctx.funcing(kFAILED)
	ctx.next(funcs...)

	//把失败处理器加入调用列表
	handlers := module.failedHandlerActions(ctx.Site)
	ctx.next(handlers...)

	//加入默认的错误处理
	ctx.next(module.failedDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
func (module *httpModule) failedDefaultHandler(ctx *Http) {
	failed := newResult(_kFAILED)
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
	funcs := ctx.funcing(kDENIED)
	ctx.next(funcs...)

	//把失败处理器加入调用列表
	handlers := module.deniedHandlerActions(ctx.Site)
	ctx.next(handlers...)

	//加入默认的错误处理
	ctx.next(module.deniedDefaultHandler)
	ctx.Next()
}

//最终还是由response处理
//如果是ajax。返回拒绝
//如果不是， 返回一个脚本提示
func (module *httpModule) deniedDefaultHandler(ctx *Http) {
	denied := newResult(_kDENIED)
	if res := ctx.Result(); res != nil {
		denied = res
	}

	if ctx.Ajax {
		ctx.Answer(denied)
	} else {
		ctx.Alert(denied)
	}
}

//可否智能判断是否跨站返回URL
func (url *httpUrl) Route(name string, values ...Map) string {

	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") ||
		strings.HasPrefix(name, "ws://") || strings.HasPrefix(name, "wss://") {
		return name
	}

	//当前站点
	currSite := ""
	if url.ctx != nil {
		currSite = url.ctx.Site
		if name == "" {
			name = url.ctx.Name
		}
	}

	params, querys, options := Map{}, Map{}, Map{}
	if len(values) > 0 {
		for k, v := range values[0] {
			if strings.HasPrefix(k, "{") && strings.HasSuffix(k, "}") {
				params[k] = v
			} else if strings.HasPrefix(k, "[") && strings.HasSuffix(k, "]") {
				options[k] = v
			} else {
				querys[k] = v
			}
		}
	}

	// justSite, justName := "", ""
	justSite := ""
	if strings.Contains(name, ".") {
		i := strings.Index(name, ".")
		justSite = name[:i]
		// justName = name[i+1:]
	}

	//如果是*.开头，因为在file.driver里可能定义*.xx.xxx.xx做为路由访问文件
	if justSite == "*" {
		if currSite != "" {
			justSite = currSite
		} else {
			//只能随机选一个站点了
			for site, _ := range Config.Site {
				justSite = site
				break
			}
		}
		name = strings.Replace(name, "*", justSite, 1)
	}

	//如果是不同站点，强制带域名
	if justSite != currSite {
		options["[site]"] = justSite
	} else if options["[site]"] != nil {
		options["[site]"] = currSite
	}

	nameget := fmt.Sprintf("%s.get", name)
	namepost := fmt.Sprintf("%s.post", name)
	var config Map

	//搜索定义
	if vv, ok := mHttp.router.chunkdata(name).(Map); ok {
		config = vv
	} else if vv, ok := mHttp.router.chunkdata(nameget).(Map); ok {
		config = vv
	} else if vv, ok := mHttp.router.chunkdata(namepost).(Map); ok {
		config = vv
	} else {
		//没有找到路由定义
		return name
	}

	if config["socket"] != nil {
		options["[socket]"] = true
	}

	uri := ""
	if vv, ok := config["uri"].(string); ok {
		uri = vv
	} else if vv, ok := config["uris"].([]string); ok && len(vv) > 0 {
		uri = vv[0]
	} else {
		return "[no uri here]"
	}

	argsConfig := Map{}
	if c, ok := config["args"].(Map); ok {
		argsConfig = c
	}

	//选项处理
	if options[URL_BACK] != nil && url.ctx != nil {
		var url = url.Back()
		querys[BACKURL] = Encrypt(url)
	}
	//选项处理
	if options[URL_LAST] != nil && url.ctx != nil {
		var url = url.Last()
		querys[BACKURL] = Encrypt(url)
	}
	//选项处理
	if options[URL_CURRENT] != nil && url.ctx != nil {
		var url = url.Current()
		querys[BACKURL] = Encrypt(url)
	}
	//自动携带原有的query信息
	if options[URL_QUERY] != nil && url.ctx != nil {
		for k, v := range url.ctx.Query {
			querys[k] = v
		}
	}

	//所以，解析uri中的参数，值得分几类：
	//1传的值，2param值, 3默认值
	//其中主要问题就是，传的值，需要到args解析，用于加密，这个值和auto值完全重叠了，除非分2次解析
	//为了框架好用，真是操碎了心
	dataValues, paramValues, autoValues := Map{}, Map{}, Map{}

	//1. 处理传过来的值
	//从value中获取
	//如果route不定义args，这里是拿不到值的
	dataArgsValues, dataParseValues := Map{}, Map{}
	for k, v := range params {
		if k[0:1] == "{" {
			k = strings.Replace(k, "{", "", -1)
			k = strings.Replace(k, "}", "", -1)
			dataArgsValues[k] = v
		} else {
			//这个也要？要不然指定的一些page啥的不行？
			dataArgsValues[k] = v
			//另外的是query的值
			querys[k] = v
		}
	}

	//上下文
	var ctx *Context
	if url.ctx != nil {
		ctx = &url.ctx.Context
	}

	dataErr := mBase.Mapping(argsConfig, dataArgsValues, dataParseValues, false, true, ctx)
	if dataErr == nil {
		for k, v := range dataParseValues {

			//注意，这里能拿到的，还有非param，所以不能直接用加{}写入
			if _, ok := params[k]; ok {
				dataValues[k] = v
			} else if _, ok := params["{"+k+"}"]; ok {
				dataValues["{"+k+"}"] = v
			} else {
				//这里是默认值应该，就不需要了
			}
		}
	}

	//所以这里还得处理一次，如果route不定义args，parse就拿不到值，就直接用values中的值
	for k, v := range params {
		if k[0:1] == "{" && dataValues[k] == nil {
			dataValues[k] = v
		}
	}

	//2.params中的值
	//从params中来一下，直接参数解析
	if url.ctx != nil {
		for k, v := range url.ctx.Param {
			paramValues["{"+k+"}"] = v
		}
	}

	//3. 默认值
	//从value中获取
	autoArgsValues, autoParseValues := Map{}, Map{}
	autoErr := mBase.Mapping(argsConfig, autoArgsValues, autoParseValues, false, true, ctx)
	if autoErr == nil {
		for k, v := range autoParseValues {
			autoValues["{"+k+"}"] = v
		}
	}

	//开始替换值
	regx := regexp.MustCompile(`\{[_\*A-Za-z0-9]+\}`)
	uri = regx.ReplaceAllStringFunc(uri, func(p string) string {
		key := strings.Replace(p, "*", "", -1)

		if v, ok := dataValues[key]; ok {
			//for query string encode/decode
			delete(dataValues, key)
			//先从传的值去取
			return fmt.Sprintf("%v", v)
		} else if v, ok := paramValues[key]; ok {
			//再从params中去取
			return fmt.Sprintf("%v", v)
		} else if v, ok := autoValues[key]; ok {
			//最后从默认值去取
			return fmt.Sprintf("%v", v)
		} else {
			//有参数没有值,
			return p
		}
	})

	//get参数，考虑一下走mapping，自动加密参数不？
	queryStrings := []string{}
	for k, v := range querys {
		sv := fmt.Sprintf("%v", v)
		if sv != "" {
			queryStrings = append(queryStrings, fmt.Sprintf("%v=%v", k, v))
		}
	}
	if len(queryStrings) > 0 {
		uri += "?" + strings.Join(queryStrings, "&")
	}

	if site, ok := options["[site]"].(string); ok && site != "" {
		uri = url.Site(site, uri, options)
	}

	return uri
}

func (url *httpUrl) Site(name string, path string, options ...Map) string {
	option := Map{}
	if len(options) > 0 {
		option = options[0]
	}

	uuu := ""
	ssl, socket := false, false

	//如果有上下文，如果是当前站点，就使用当前域
	if url.ctx != nil && url.ctx.Site == name {
		uuu = url.ctx.Host
		if vv, ok := Config.Site[name]; ok {
			ssl = vv.Ssl
		}
	} else if vv, ok := Config.Site[name]; ok {
		ssl = vv.Ssl
		if len(vv.Hosts) > 0 {
			uuu = vv.Hosts[0]
		}
	} else {
		uuu = "127.0.0.1"
		//uuu = fmt.Sprintf("127.0.0.1:%v", Config.Http.Port)
	}

	if Mode == Developing && Config.Http.Port != 80 {
		uuu = fmt.Sprintf("%s:%d", uuu, Config.Http.Port)
	}

	if option["[ssl]"] != nil {
		ssl = true
	}
	if option["[socket]"] != nil {
		socket = true
	}

	if socket {
		if ssl {
			uuu = "wss://" + uuu
		} else {
			uuu = "ws://" + uuu
		}
	} else {
		if ssl {
			uuu = "https://" + uuu
		} else {
			uuu = "http://" + uuu
		}
	}

	if path != "" {
		return fmt.Sprintf("%s%s", uuu, path)
	} else {
		return uuu
	}
}

func (url *httpUrl) Backing() bool {
	if url.req == nil {
		return false
	}

	if s, ok := url.ctx.Query[BACKURL]; ok && s != "" {
		return true
	} else if url.req.Referer() != "" {
		return true
	}
	return false
}

func (url *httpUrl) Back() string {
	if url.ctx == nil {
		return "/"
	}

	if s, ok := url.ctx.Query[BACKURL].(string); ok && s != "" {
		return Decrypt(s)
	} else if url.ctx.Header("referer") != "" {
		return url.ctx.Header("referer")
	} else {
		//都没有，就是当前URL
		return url.Current()
	}
}

func (url *httpUrl) Last() string {
	if url.req == nil {
		return "/"
	}

	if ref := url.req.Referer(); ref != "" {
		return ref
	} else {
		//都没有，就是当前URL
		return url.Current()
	}
}

func (url *httpUrl) Current() string {
	if url.req == nil {
		return "/"
	}

	// return url.req.URL.String()

	// return fmt.Sprintf("%s://%s%s", url.req.URL., url.req.Host, url.req.URL.RequestURI())

	return url.Site(url.ctx.Site, url.req.URL.RequestURI())
}

//为了view友好，expires改成Any，支持duration解析
func (url *httpUrl) Download(target Any, name string, args ...Any) string {
	if url.ctx == nil {
		return ""
	}

	if coding, ok := target.(string); ok && coding != "" {

		if strings.HasPrefix("http://", coding) || strings.HasPrefix("https://", coding) || strings.HasPrefix("ftp://", coding) {
			return coding
		}

		expires := []time.Duration{}
		if len(args) > 0 {
			switch vv := args[0].(type) {
			case int:
				expires = append(expires, time.Second*time.Duration(vv))
			case time.Duration:
				expires = append(expires, vv)
			case string:
				if dd, ee := util.ParseDuration(vv); ee == nil {
					expires = append(expires, dd)
				}
			}
		}

		return safeBrowse(coding, name, url.ctx.Id, url.ctx.Ip(), expires...)

		//url.ctx.lastError = nil
		//if uuu, err := mFile.Browse(coding, name, aaaaa, expires...); err != nil {
		//	url.ctx.lastError = errResult(err)
		//	return ""
		//} else {
		//	return uuu
		//}
	}

	return "[无效下载]"
}
func (url *httpUrl) Browse(target Any, args ...Any) string {
	if url.ctx == nil {
		return ""
	}

	if coding, ok := target.(string); ok && coding != "" {

		if strings.HasPrefix("http://", coding) || strings.HasPrefix("https://", coding) || strings.HasPrefix("ftp://", coding) {
			return coding
		}

		expires := []time.Duration{}
		if len(args) > 0 {
			switch vv := args[0].(type) {
			case int:
				expires = append(expires, time.Second*time.Duration(vv))
			case time.Duration:
				expires = append(expires, vv)
			case string:
				if dd, ee := util.ParseDuration(vv); ee == nil {
					expires = append(expires, dd)
				}
			}
		}

		return safeBrowse(coding, "", url.ctx.Id, url.ctx.Ip(), expires...)
		//url.ctx.lastError = nil
		//if uuu, err := mFile.Browse(coding, "", aaaaa, expires...); err != nil {
		//	url.ctx.lastError = errResult(err)
		//	return ""
		//} else {
		//	return uuu
		//}
	}

	return "[无效文件]"
}
func (url *httpUrl) Preview(target Any, width, height, tttt int64, args ...Any) string {
	if url.ctx == nil {
		return ""
	}

	if coding, ok := target.(string); ok && coding != "" {

		if strings.HasPrefix("http://", coding) || strings.HasPrefix("https://", coding) || strings.HasPrefix("ftp://", coding) {
			return coding
		}

		expires := []time.Duration{}
		if len(args) > 0 {
			switch vv := args[0].(type) {
			case int:
				expires = append(expires, time.Second*time.Duration(vv))
			case time.Duration:
				expires = append(expires, vv)
			case string:
				if dd, ee := util.ParseDuration(vv); ee == nil {
					expires = append(expires, dd)
				}
			}
		}

		return safePreview(coding, width, height, tttt, url.ctx.Id, url.ctx.Ip(), expires...)

	}

	return "/nothing.png"
}
