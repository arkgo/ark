package ark

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arkgo/asset/util"
	. "github.com/arkgo/base"
)

type (
	httpUrl struct {
		ctx *Http
		req *http.Request
	}
)

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
