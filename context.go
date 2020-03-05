package ark

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/arkgo/asset/toml"
	"github.com/arkgo/asset/util"
	. "github.com/arkgo/base"
)

type (
	context interface {
		Lang() string
		Zone() *time.Location
		Data(bases ...string) DataBase
	}

	HttpHeaders = map[string]string
	HttpCookies = map[string]http.Cookie

	HttpFunc func(*Http)
	Http     struct {
		index     int        //下一个索引
		nexts     []HttpFunc //方法列表
		lastError *Res

		thread   HttpThread
		requqest *http.Request
		response http.ResponseWriter

		databases map[string]DataBase

		Charset string
		Lang    string
		Zone    *time.Location

		Id         string
		Name       string
		Config     Map
		siteConfig SiteConfig

		Site   string //站点key
		Method string //请求方法，大写
		Host   string //请求域名
		Path   string //请求路径
		Uri    string //请求uri

		headers HttpHeaders
		cookies HttpCookies
		// sessions HttpSessions

		Params Map //uri 中的参数
		Query  Map //querystring 参数
		Form   Map //表单参数
		Upload Map //上传文件数据
		Data   Map //viewdata传给视图

		Value Map //所有参数汇总
		Args  Map //定义的args解析后的参数
		Auth  Map //auth会话校验对象，支持查询数据库
		Item  Map //查询单个数据库对象
		Local Map //上下文传递数据

		Code int    //返回HTTP状态
		Type string //返回内容类型
		Body Any    //返回body

		Url *httpUrl
	}
)

func httpContext(thread HttpThread) *Http {
	ctx := &Http{
		index: 0, nexts: make([]HttpFunc, 0), databases: make(map[string]DataBase),
		Charset: UTF8, Lang: DEFAULT, Zone: time.Local,
		thread: thread, requqest: thread.Request(), response: thread.Response(),
		headers: make(HttpHeaders), cookies: make(HttpCookies),
		Params: make(Map), Query: make(Map), Form: make(Map), Upload: make(Map), Data: make(Map),
		Value: make(Map), Args: make(Map), Auth: make(Map), Item: make(Map), Local: make(Map),
	}

	ctx.Name = thread.Name()
	ctx.Site = thread.Site()

	ctx.Method = strings.ToUpper(ctx.request.Method)
	ctx.Uri = ctx.request.RequestURI
	ctx.Path = ctx.request.URL.Path

	//使用域名去找site
	ctx.Host = ctx.request.Host
	if strings.Contains(ctx.Host, ":") {
		hosts := strings.Split(ctx.Host, ":")
		if len(hosts) > 0 {
			ctx.Host = hosts[0]
		}
	}
	if ctx.Site == "" {
		if site, ok := Config.hosts[ctx.Host]; ok {
			ctx.Site = site
		}
	}

	if vvvv, ookk := Config.Site[ctx.Site]; ookk {
		ctx.siteConfig = vvvv
	} else {
		ctx.siteConfig = SiteConfig{}
	}

	//获取根域名
	ctx.domain = ctx.request.Host
	if strings.Contains(ctx.domain, ":") {
		ctx.domain = strings.Split(ctx.domain, ":")[0]
	}
	parts := strings.Split(ctx.domain, ".")
	if len(parts) >= 2 {
		l := len(parts)
		ctx.domain = parts[l-2] + "." + parts[l-1]
	}

	ctx.Url = &httpUrl{
		ctx: ctx, req: ctx.request,
	}

	return ctx
}

//最终的清理工作
func (ctx *Http) final() {
	for _, base := range ctx.databases {
		base.Close()
	}
}
func (ctx *Http) clear() {
	ctx.index = 0
	ctx.nexts = make([]HttpFunc, 0)
}
func (ctx *Http) next(nexts ...HttpFunc) {
	ctx.nexts = append(ctx.nexts, nexts...)
}
func (ctx *Http) funcing(key string) []HttpFunc {
	funcs := []HttpFunc{}

	if action, ok := ctx.Config[key]; ok && action != nil {
		switch actions := action.(type) {
		case func(*Http):
			funcs = append(funcs, actions)
		case []func(*Http):
			for _, action := range actions {
				funcs = append(funcs, action)
			}
		case HttpFunc:
			funcs = append(funcs, actions)
		case []HttpFunc:
			funcs = append(funcs, actions...)
		default:
		}
	}

	return funcs
}
func (ctx *Http) Next() {
	if len(ctx.nexts) > ctx.index {
		next := ctx.nexts[ctx.index]
		ctx.index++
		if next != nil {
			next(ctx)
		} else {
			ctx.Next()
		}
	} else {
		//是否需要做执行完的处理
	}
}

func (ctx *Http) sessional(defs ...bool) bool {
	sessional := false
	if len(defs) > 0 {
		sessional = defs[0]
	}

	if vv, ok := ctx.Config["session"].(bool); ok {
		sessional = vv
	}

	//如果有auth节，强制使用session
	if _, ok := ctx.Config["auth"]; ok {
		sessional = true
	}

	return sessional
}

//客户端请求校验
//接口请求校验
//设备，系统，版本，客户端，版本号，时间戳，签名
//{device}/{system}/{version}/{client}/{number}/{time}/{sign}
func (ctx *Http) clientHandler() *Res {
	//var req = ctx.req.Reader

	checking := true
	coding := "text"

	if ctx.siteConfig.Validate == "" {
		checking = false
	} else {
		coding = ctx.siteConfig.Validate
	}

	//个别路由通行
	if vv, ok := ctx.Config["passport"].(bool); ok && vv {
		checking = false
	}
	if vv, ok := ctx.Config["validate"].(bool); ok && vv == false {
		checking = false
	}

	cs := ""
	if vv := ctx.Header("client"); vv != "" {
		cs = strings.TrimSpace(vv)
	}

	if cs == "" {
		if checking {
			return Invalid
		} else {
			return nil
		}
	}

	args := Map{
		"client": Map{"type": "string", "must": true, "decode": coding},
	}
	data := Map{
		"client": cs,
	}
	value := Map{}
	err := mBase.Mapping(args, data, value, false, false, &ctx.Context)
	if err != nil {
		return Invalid
	}

	//return nil

	client := value["client"].(string)
	vals := strings.Split(client, "/")
	if len(vals) < 7 && checking {
		//Debug("client", "Length", err, client)
		return Invalid
	}

	format := `{device}/{system}/{version}/{client}/{number}/{time}/{path}`
	if ctx.siteConfig.Format != "" {
		format = ctx.siteConfig.Format
	}

	format = strings.Replace(format, "{device}", vals[0], -1)
	format = strings.Replace(format, "{system}", vals[1], -1)
	format = strings.Replace(format, "{version}", vals[2], -1)
	format = strings.Replace(format, "{client}", vals[3], -1)
	format = strings.Replace(format, "{number}", vals[4], -1)
	format = strings.Replace(format, "{time}", vals[5], -1)
	format = strings.Replace(format, "{path}", ctx.Path, -1)

	sign := strings.ToLower(util.Md5(format))

	//Debug("vvv", sign, format, value)

	if sign != vals[6] && checking {
		//Debug("ClientSign", ctx.Uri, sign, data["client"], value["client"])
		Debug("format", format)
		Debug("sign", sign)
		Debug("path", ctx.Uri)
		Debug("client", value["client"])
		return Invalid
	}

	//保存参数
	ctx.Client["device"] = vals[0]
	ctx.Client["system"] = vals[1]
	ctx.Client["version"] = vals[2]
	ctx.Client["client"] = vals[3]
	ctx.Client["number"] = vals[4]
	ctx.Client["time"] = vals[5]
	ctx.Client["sign"] = vals[6]

	//到这里才成功
	return nil
}

//专门处理base64格式的文件上传
func (ctx *Http) formUploadHandler(values []string) []Map {
	files := []Map{}

	baseExp := regexp.MustCompile(`data\:(.*)\;base64,(.*)`)
	for _, base := range values {
		arr := baseExp.FindStringSubmatch(base)
		if len(arr) == 3 {
			baseBytes, err := base64.StdEncoding.DecodeString(arr[2])
			if err == nil {
				h := sha1.New()
				if _, err := h.Write(baseBytes); err == nil {
					hash := fmt.Sprintf("%x", h.Sum(nil))

					mimeType := arr[1]
					extension := mBase.Typemime(mimeType)
					filename := fmt.Sprintf("%s.%s", hash, extension)
					length := len(baseBytes)

					//保存临时文件
					tempfile := path.Join(Config.Path.Upload, fmt.Sprintf("%s_%s", Name, hash))
					if extension != "" {
						tempfile = fmt.Sprintf("%s.%s", tempfile, extension)
					}

					if save, err := os.OpenFile(tempfile, os.O_WRONLY|os.O_CREATE, 0777); err == nil {
						defer save.Close()
						if _, err := save.Write(baseBytes); err == nil {
							files = append(files, Map{
								//"hash":      hash,
								"filename":  filename,
								"extension": strings.ToLower(extension),
								"mimetype":  mimeType,
								"length":    length,
								"tempfile":  tempfile,
							})
						}
					}
				}
			}
		}
	}

	return files
}

func (ctx *Http) formHandler() *Res {
	var req = ctx.req.Reader

	//URL中的参数
	for k, v := range ctx.req.Params {
		ctx.Param[k] = v
		ctx.Value[k] = v
	}

	//urlquery
	for k, v := range req.URL.Query() {
		if len(v) == 1 {
			ctx.Query[k] = v[0]
			ctx.Value[k] = v[0]
		} else if len(v) > 1 {
			ctx.Query[k] = v
			ctx.Value[k] = v
		}
	}

	//是否AJAX请求，可能在拦截器里手动指定为true了，就不处理了
	if ctx.Ajax == false {
		if ctx.Header("X-Requested-With") != "" {
			ctx.Ajax = true
		} else if ctx.Header("Ajax") != "" {
			ctx.Ajax = true
		} else {
			ctx.Ajax = false
		}
	}

	//客户端的默认语言
	if al := ctx.Header("Accept-Language"); al != "" {
		accepts := strings.Split(al, ",")
		if len(accepts) > 0 {
		llll:
			for _, accept := range accepts {
				if i := strings.Index(accept, ";"); i > 0 {
					accept = accept[0:i]
				}
				//遍历匹配
				for lang, config := range Config.Lang {
					for _, acccc := range config.Accepts {
						if strings.ToLower(acccc) == strings.ToLower(accept) {
							ctx.Lang = lang
							break llll
						}
					}
				}
			}
		}
	}

	uploads := map[string][]Map{}

	//if ctx.Method == "POST" || ctx.Method == "PUT" || ctx.Method == "DELETE" || ctx.Method == "PATCH" {
	if ctx.Method != "GET" {
		//根据content-type来处理
		ctype := ctx.Header("Content-Type")
		if strings.Contains(ctype, "json") {
			body, err := ioutil.ReadAll(req.Body)
			if err == nil {
				ctx.Body = RawBody(body)

				m := Map{}
				err := json.Unmarshal(body, &m)
				if err == nil {
					//遍历JSON对象
					for k, v := range m {
						ctx.Form[k] = v
						ctx.Value[k] = v

						if vs, ok := v.(string); ok {
							baseFiles := module.formUploadHandler([]string{vs})
							if len(baseFiles) > 0 {
								uploads[k] = baseFiles
							}
						} else if vs, ok := v.([]Any); ok {
							vsList := []string{}
							for _, vsa := range vs {
								if vss, ok := vsa.(string); ok {
									vsList = append(vsList, vss)
								}
							}

							if len(vsList) > 0 {
								baseFiles := module.formUploadHandler(vsList)
								if len(baseFiles) > 0 {
									uploads[k] = baseFiles
								}
							}
						}

					}
				}
			}
		} else if strings.Contains(ctype, "xml") {
			body, err := ioutil.ReadAll(req.Body)
			if err == nil {
				ctx.Body = RawBody(body)

				m := Map{}
				err := xml.Unmarshal(body, &m)
				if err == nil {
					//遍历XML对象
					for k, v := range m {
						ctx.Form[k] = v
						ctx.Value[k] = v
					}
				}
			}
		} else {

			// if ctype=="application/x-www-form-urlencoded" || ctype=="multipart/form-data" {
			// }

			err := req.ParseMultipartForm(32 << 20)
			if err != nil {
				//表单解析有问题，就处理成原始STRING
				body, err := ioutil.ReadAll(req.Body)
				if err == nil {
					ctx.Body = RawBody(body)
				}

			}

			names := []string{}
			values := url.Values{}
			// uploads := map[string][]Map{}

			if req.MultipartForm != nil {

				//处理表单，这里是否应该直接写入ctx.Form比较好？
				for k, v := range req.MultipartForm.Value {
					//有个问题，当type=file时候，又不选文件的时候，value里会存在一个空字串的value
					//如果同一个form name 有多条记录，这时候会变成一个[]string，的空串数组
					//这时候，mapping解析文件的时候[file]就会出问题，会判断文件类型，这时候是[]string就出问题了
					// ctx.Form[k] = v
					names = append(names, k)
					values[k] = v
				}

				//FILE可能要弄成JSON，文件保存后，MIME相关的东西，都要自己处理一下
				for k, v := range req.MultipartForm.File {
					//这里应该保存为数组
					files := []Map{}

					//处理多个文件
					for _, f := range v {

						if f.Size <= 0 || f.Filename == "" {
							continue
						}

						hash := ""
						filename := f.Filename
						mimetype := f.Header.Get("Content-Type")
						extension := strings.ToLower(path.Ext(filename))
						if extension != "" {
							extension = extension[1:] //去掉点.
						}

						var tempfile string
						var length int64 = f.Size

						//先计算hash
						if file, err := f.Open(); err == nil {

							h := sha1.New()
							if _, err := io.Copy(h, file); err == nil {

								hash = fmt.Sprintf("%x", h.Sum(nil))

								//保存临时文件
								tempfile = path.Join(Config.Path.Upload, fmt.Sprintf("%s_%s", Name, hash))
								if extension != "" {
									tempfile = fmt.Sprintf("%s.%s", tempfile, extension)
								}

								//重新定位
								file.Seek(0, 0)

								if save, err := os.OpenFile(tempfile, os.O_WRONLY|os.O_CREATE, 0777); err == nil {
									io.Copy(save, file) //保存文件
									save.Close()

									msg := Map{
										"hash":      hash,
										"filename":  filename,
										"extension": extension,
										"mimetype":  mimetype,
										"length":    length,
										"tempfile":  tempfile,
									}

									files = append(files, msg)
								}

							}

							//最后关闭文件
							file.Close()
						}

						uploads[k] = files
					}
				}

			} else if req.PostForm != nil {
				for k, v := range req.PostForm {
					names = append(names, k)
					values[k] = v
				}

			} else if req.Form != nil {
				for k, v := range req.Form {
					names = append(names, k)
					values[k] = v
				}
			}

			tomlroot := map[string][]string{}
			tomldata := map[string]map[string][]string{}

			//顺序很重要
			tomlexist := map[string]bool{}
			tomlnames := []string{}

			//统一解析
			for _, k := range names {
				v := values[k]

				//写入form
				if len(v) == 1 {
					ctx.Form[k] = v[0]
				} else if len(v) > 1 {
					ctx.Form[k] = v
				}

				//解析base64文件 begin
				baseFiles := module.formUploadHandler([]string(v))
				if len(baseFiles) > 0 {
					uploads[k] = baseFiles
				}
				//解析base64文件 end

				// key := fmt.Sprintf("value[%s]", k)
				// forms[k] = v

				if strings.Contains(k, ".") {

					//以最后一个.分割，前为key，后为field
					i := strings.LastIndex(k, ".")
					key := k[:i]
					field := k[i+1:]

					if vv, ok := tomldata[key]; ok {
						vv[field] = v
					} else {
						tomldata[key] = map[string][]string{
							field: v,
						}
					}

					if _, ok := tomlexist[key]; ok == false {
						tomlexist[key] = true
						tomlnames = append(tomlnames, key)
					}

				} else {
					tomlroot[k] = v
				}

				//这里不写入， 解析完了才
				// ctx.Value[k] = v
			}

			lines := []string{}
			for kk, vv := range tomlroot {
				if len(vv) > 1 {
					lines = append(lines, fmt.Sprintf(`%s = ["""%s"""]`, kk, strings.Join(vv, `""","""`)))
				} else {
					lines = append(lines, fmt.Sprintf(`%s = """%s"""`, kk, vv[0]))
				}
			}
			for _, kk := range tomlnames {
				vv := tomldata[kk]

				//普通版
				// lines = append(lines, fmt.Sprintf("[%s]", kk))
				// for ff,fv := range vv {
				// 	if len(fv) > 1 {
				// 		lines = append(lines, fmt.Sprintf(`%s = ["%s"]`, ff, strings.Join(fv, `","`)))
				// 	} else {
				// 		lines = append(lines, fmt.Sprintf(`%s = "%s"`, ff, fv[0]))
				// 	}
				// }

				//数组版
				//先判断一下，是不是map数组
				length := 0
				for _, fv := range vv {
					if length == 0 {
						length = len(fv)
					} else {
						if length != len(fv) {
							length = -1
							break
						}
					}
				}

				//如果length>1是数组，并且相等
				if length > 1 {
					for i := 0; i < length; i++ {
						lines = append(lines, fmt.Sprintf("[[%s]]", kk))
						for ff, fv := range vv {
							lines = append(lines, fmt.Sprintf(`%s = """%s"""`, ff, fv[i]))
						}
					}

				} else {
					lines = append(lines, fmt.Sprintf("[%s]", kk))
					for ff, fv := range vv {
						if len(fv) > 1 {
							lines = append(lines, fmt.Sprintf(`%s = ["""%s"""]`, ff, strings.Join(fv, `""","""`)))
						} else {
							lines = append(lines, fmt.Sprintf(`%s = """%s"""`, ff, fv[0]))
						}
					}
				}
			}

			value := Map{}
			_, err = toml.Decode(strings.Join(lines, "\n"), &value)
			if err == nil {
				for k, v := range value {
					ctx.Value[k] = v
				}
			} else {
				for k, v := range values {
					if len(v) == 1 {
						ctx.Value[k] = v[0]
					} else if len(v) > 1 {
						ctx.Value[k] = v
					}
				}
			}
		}
	}

	for k, v := range uploads {
		if len(v) == 1 {
			ctx.Value[k] = v[0]
			ctx.Upload[k] = v[0]
		} else if len(v) > 1 {
			ctx.Value[k] = v
			ctx.Upload[k] = v
		}
	}

	return nil
}

//处理参数
func (ctx *Http) argsHandler() *Res {

	//argn表示参数都可为空
	argn := false
	if v, ok := ctx.Config["argn"].(bool); ok {
		argn = v
	}

	if argsConfig, ok := ctx.Config["args"].(Map); ok {
		argsValue := Map{}
		err := mBase.Mapping(argsConfig, ctx.Value, argsValue, argn, false, ctx)
		if err != nil {
			return err
		}

		for k, v := range argsValue {
			ctx.Args[k] = v
		}
	}

	return nil
}

//处理认证
func (ctx *Http) authHandler() *Res {

	if auths, ok := ctx.Config[kAUTH].(Map); ok {
		saveMap := Map{}

		for authKey, authMap := range auths {

			if _, ok := authMap.(Map); ok == false {
				continue
			}

			ohNo := false
			authConfig := authMap.(Map)

			if vv, ok := authConfig[kSIGN].(string); ok == false || vv == "" {
				continue
			}

			authSign := authConfig[kSIGN].(string)
			authMust := false
			// authName := authSign

			if vv, ok := authConfig[kMUST].(bool); ok {
				authMust = vv
			}
			// if vv,ok := authConfig[kNAME].(string); ok && vv!="" {
			// 	authName = vv
			// }

			//判断是否登录
			if ctx.Signed(authSign) {

				//支持两种方式
				//1. data=table 如： "auth": "db.user"
				//2. base=db, table=user

				base, table := "", ""
				if authConfig["data"] != nil {
					if vv, ok := authConfig["data"].(string); ok {
						i := strings.Index(vv, ".")
						base = vv[:i]
						table = vv[i+1:]
					}
				} else if authConfig["base"] != nil && authConfig["table"] != nil {
					if vv, ok := authConfig["base"].(string); ok {
						base = vv
					}
					if vv, ok := authConfig["table"].(string); ok {
						table = vv
					}
				}

				if base != "" && table != "" {
					db := ctx.dataBase(base)
					id := ctx.Signal(authSign)
					item := db.Table(table).Entity(id)

					if item == nil {
						if authMust {
							if vv, ok := authConfig[kERROR].(*Res); ok {
								return vv
							} else {
								errKey := ".auth.error"
								if vv, ok := authConfig[kERROR].(string); ok {
									errKey = vv
								}
								return newResult(errKey + "." + authKey)
							}
						}
					} else {
						saveMap[authKey] = item
					}
				}

			} else {
				ohNo = true
			}

			//到这里是未登录的
			//而且是必须要登录，才显示错误
			if ohNo && authMust {
				if vv, ok := authConfig[kEMPTY].(*Res); ok {
					return vv
				} else {
					errKey := ".auth.empty"
					if vv, ok := authConfig[kEMPTY].(string); ok {
						errKey = vv
					}
					return newResult(errKey + "." + authKey)
				}
			}
		}

		//存入
		for k, v := range saveMap {
			ctx.Auth[k] = v
		}
	}

	return nil
}

//Entity实体处理
func (ctx *Http) itemHandler() *Res {

	if cfg, ok := ctx.Config[kITEM].(Map); ok {
		saveMap := Map{}

		for itemKey, v := range cfg {
			if config, ok := v.(Map); ok {

				//是否必须
				must := true
				if vv, ok := config[kMUST].(bool); ok {
					must = vv
				}

				//itemName := itemKey
				//if vv,ok := config[kNAME].(string); ok && vv != "" {
				//	itemName = vv
				//}

				realKey := itemKey
				var realVal Any
				if vv, ok := config[kARGS].(string); ok {
					realKey = vv
					realVal = ctx.Args[realKey]
					//} else if vv, ok := config[kPARAM].(string); ok {
					//	realKey = vv
					//	realVal = ctx.Param[realKey]
					//} else if vv, ok := config[kQUERY].(string); ok {
					//	realKey = vv
					//	realVal = ctx.Query[realKey]
				} else if vv, ok := config[kVALUE].(string); ok {
					realKey = vv
					realVal = ctx.Value[realKey]
				} else if vv, ok := config[kKEY].(string); ok {
					realKey = vv
					realVal = ctx.Value[realKey]
				} else {
					realVal = nil
				}

				if realVal == nil && must {
					if vv, ok := config[kEMPTY].(*Res); ok {
						return vv
					} else {
						errKey := ".item.empty"
						if vv, ok := config[kEMPTY].(string); ok {
							errKey = vv
						}
						return newResult(errKey + "." + itemKey)
					}
				} else {

					//支持两种方式
					//1. data=table 如： "auth": "db.user"
					//2. base=db, table=user

					base, table := "", ""
					if config["data"] != nil {
						if vv, ok := config["data"].(string); ok {
							i := strings.Index(vv, ".")
							base = vv[:i]
							table = vv[i+1:]
						}
					} else if config["base"] != nil && config["table"] != nil {
						if vv, ok := config["base"].(string); ok {
							base = vv
						}
						if vv, ok := config["table"].(string); ok {
							table = vv
						}
					}

					//判断是否需要查询数据
					if base != "" && table != "" {
						//要查询库
						db := ctx.dataBase(base)
						item := db.Table(table).Entity(realVal)
						if must && (db.Erred() != nil || item == nil) {
							if vv, ok := config[kERROR].(*Res); ok {
								return vv
							} else {
								errKey := ".item.error"
								if vv, ok := config[kERROR].(string); ok {
									errKey = vv
								}
								return newResult(errKey + "." + itemKey)
							}
						} else {
							saveMap[itemKey] = item
						}
					}

				}
			}
		}

		//存入
		for k, v := range saveMap {
			ctx.Item[k] = v
		}
	}
	return nil
}

//返回最后的错误信息
//获取操作结果
func (ctx *Http) Result() *Res {
	err := ctx.lastError
	ctx.lastError = nil
	return err
}

//接入错误处理流程，和模块挂钩了
func (ctx *Http) Found() {
	ark.Http.found(ctx)
}
func (ctx *Http) Error(res *Res) {
	ark.Http.error(ctx)
}
func (ctx *Http) Failed(res *Res) {
	ctx.lastError = res
	ark.Http.failed(ctx)
}
func (ctx *Http) Denied(res *Res) {
	ctx.lastError = res
	ark.Http.denied(ctx)
}

//通用方法
func (ctx *Http) Header(key string, vals ...string) string {
	if len(vals) > 0 {
		ctx.headers[key] = vals[0]
		return vals[0]
	} else {
		//读header
		return ctx.req.Reader.Header.Get(key)
	}
}

//通用方法
func (ctx *Http) Cookie(key string, vals ...Any) string {
	//这个方法同步加密
	if len(vals) > 0 {

		//设置header
		switch val := vals[0].(type) {
		case http.Cookie:
			// val.Value = url.QueryEscape(val.Value)
			val.Value = Encrypt(val.Value)
			ctx.cookies[key] = val
		case string:
			cookie := http.Cookie{Name: key, Value: val, Path: "/", HttpOnly: true}
			cookie.Value = Encrypt(cookie.Value)
			ctx.cookies[key] = cookie
		default:
			return ""
		}

	} else {
		//读cookie
		c, e := ctx.req.Reader.Cookie(key)
		if e == nil {
			//解密cookie，这里解密，为什么到最后才加密， 应该一起加解密，写入的时候不处理了
			return Decrypt(c.Value)
		}
	}
	return ""
}

//远程存储代理
func (ctx *Http) Remote(code string, names ...string) {
	//判断处理，是文件系统，还是存储系统
	coding := Decoding(code)
	if coding == nil {
		ctx.Found()
		return
	}

	if coding.isFile() {
		ctx.Download(code, names...)
	} else {

		name := ""
		if len(names) > 0 {
			name = names[0]
		}

		sb := ctx.Store(coding.Base)
		url := sb.Browse(code, name)
		if err := sb.Erred(); err != nil {
			ctx.Error(errResult(err))
		} else {

			//因为apk文件ipfs会变成zip，所以要单独处理一下
			if coding.Type == "apk" {
				ctx.Download(code, names...)
			} else {
				ctx.Proxy(url)
			}
		}
	}
}
func (ctx *Http) Download(code string, names ...string) {

	//判断处理，是文件系统，还是存储系统
	coding := Decoding(code)
	if coding == nil {
		ctx.Found()
		return
	}

	if coding.isFile() {
		//老的文件系统

		file := mFile.Download(code)
		if file == "" {
			ctx.Error(Fail)
		} else {
			ctx.File(file, coding.Type, names...)
		}

	} else {
		//新的存储系统
		sb := ctx.Store(coding.Base)
		file := sb.Download(code)
		if err := sb.Erred(); err != nil {
			ctx.Error(errResult(err))
		} else {
			//无图片指定默认文件名和扩展名，这样比较好
			if len(names) == 0 && coding.Type != "" && coding.IsImage() == false {
				names = append(names, coding.Hash+"."+coding.Type)
			} else {
				//自动加扩展名
				if strings.HasSuffix(names[0], coding.Type) == false && coding.Type != "" {
					names[0] += "." + coding.Type
				}
			}

			ctx.File(file, coding.Type, names...)
		}
	}
}

//生成并返回缩略图
func (ctx *Http) Thumbnail(code string, width, height, tttt int64) {
	file, data, err := mFile.Thumbnail(code, width, height, tttt)
	if err != nil {
		//ctx.Error(errResult(err))
		ctx.File(path.Join(Config.Path.Static, "shared", "nothing.png"), "png")
	} else {
		ctx.File(file, data.Type)
	}
}

func (ctx *Http) Goto(url string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	ctx.Body = httpGotoBody{url}
}
func (ctx *Http) Goback() {
	url := ctx.Url.Back()
	ctx.Goto(url)
}
func (ctx *Http) Text(text Any, types ...string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	real := ""
	if res, ok := text.(*Res); ok {
		real = ctx.String(res.Text, res.Args...)
	} else if vv, ok := text.(string); ok {
		real = vv
	} else {
		real = fmt.Sprintf("%v", text)
	}

	if len(types) > 0 {
		ctx.Type = types[0]
	} else {
		ctx.Type = "text"
	}
	ctx.Body = httpTextBody{real}
}
func (ctx *Http) Html(html string, codes ...int) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	if len(codes) > 0 {
		ctx.Code = codes[0]
	}
	ctx.Type = "html"
	ctx.Body = httpHtmlBody{html}
}
func (ctx *Http) Script(script string, codes ...int) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	if len(codes) > 0 {
		ctx.Code = codes[0]
	}
	ctx.Type = "script"
	ctx.Body = httpScriptBody{script}
}
func (ctx *Http) Json(json Any, codes ...int) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	if len(codes) > 0 {
		ctx.Code = codes[0]
	}
	ctx.Type = "json"
	ctx.Body = httpJsonBody{json}
}
func (ctx *Http) Jsonp(callback string, json Any, codes ...int) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	if len(codes) > 0 {
		ctx.Code = codes[0]
	}
	ctx.Type = "jsonp"
	ctx.Body = httpJsonpBody{json, callback}
}
func (ctx *Http) Xml(xml Any, codes ...int) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	if len(codes) > 0 {
		ctx.Code = codes[0]
	}
	ctx.Type = "xml"
	ctx.Body = httpXmlBody{xml}
}

func (ctx *Http) File(file string, mimeType string, names ...string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	name := ""
	if len(names) > 0 {
		name = names[0]
	}
	if mimeType != "" {
		ctx.Type = mimeType
	} else {
		ctx.Type = "file"
	}
	ctx.Body = httpFileBody{file, name}
}

func (ctx *Http) Buffer(rd io.ReadCloser, mimeType string, names ...string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	name := ""
	if len(names) > 0 {
		name = names[0]
	}

	ctx.Code = http.StatusOK
	if mimeType != "" {
		ctx.Type = mimeType
	} else {
		ctx.Type = "file"
	}
	ctx.Body = httpBufferBody{rd, name}
}
func (ctx *Http) Down(bytes []byte, mimeType string, names ...string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	ctx.Code = http.StatusOK
	if mimeType != "" {
		ctx.Type = mimeType
	} else {
		ctx.Type = "file"
	}
	name := ""
	if len(names) > 0 {
		name = names[0]
	}
	ctx.Body = httpDownBody{bytes, name}
}

func (ctx *Http) View(view string, types ...string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	ctx.Type = "html"
	if len(types) > 0 {
		ctx.Type = types[0]
	}
	ctx.Body = httpViewBody{view, Map{}}
}

func (ctx *Http) Proxy(remote string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	u, e := url.Parse(remote)
	if e != nil {
		ctx.Error(errResult(e))
	} else {
		ctx.Body = httpProxyBody{u}
	}
}

func (ctx *Http) Route(name string, values ...Map) {
	url := ctx.Url.Route(name, values...)
	ctx.Redirect(url)
}

func (ctx *Http) Redirect(url string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	ctx.Goto(url)
}

func (ctx *Http) Alert(res *Res, urls ...string) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	code := mBase.Code(res.Text, res.Code)
	text := ctx.String(res.Text, res.Args...)

	if code == 0 {
		ctx.Code = http.StatusOK
	} else {
		//考虑到默认也200
		//ctx.Code = http.StatusInternalServerError
	}

	if len(urls) > 0 {
		text = fmt.Sprintf(`<script type="text/javascript">alert("%s"); location.href="%s";</script>`, text, urls[0])
	} else {
		text = fmt.Sprintf(`<script type="text/javascript">alert("%s"); history.back();</script>`, text)
	}
	ctx.Script(text)
}

//展示通用的提示页面
func (ctx *Http) Show(res *Res, urls ...string) {
	code := mBase.Code(res.Text, res.Code)
	text := ctx.String(res.Text, res.Args...)

	if res.OK() {
		ctx.Code = http.StatusOK
	} else {
		//考虑默认200
		//ctx.Code = http.StatusInternalServerError
	}

	m := Map{
		"code": code,
		"text": text,
		"url":  "",
	}
	if len(urls) > 0 {
		m["url"] = urls[0]
	}

	ctx.Data[kSHOW] = m
	ctx.View(kSHOW)
}

//返回操作结果，表示成功
//比如，登录，修改密码，等操作类的接口， 成功的时候，使用这个，
//args表示返回给客户端的data
//data 强制改为json格式，因为data有统一加密的可能
//所有数组都要加密。
func (ctx *Http) Answer(res *Res, args ...Map) {
	//如果已经存在了httpDownBody，那还要把原有的reader关闭
	//释放资源， 当然在file.base.close中也应该关闭已经打开的资源
	if vv, ok := ctx.Body.(httpBufferBody); ok {
		vv.buffer.Close()
	}

	code := 0
	text := ""
	if res != nil {
		code = mBase.Code(res.Text, res.Code)
		text = ctx.String(res.Text, res.Args...)
	}

	if code == 0 {
		ctx.Code = http.StatusOK
	} else {
		//考虑到，默认也200
		//ctx.Code = http.StatusInternalServerError
	}

	var data Map
	if len(args) > 0 {
		data = args[0]
	}

	//if len(args) > 0 {
	//	for k, v := range args[0] {
	//		ctx.Data[k] = v
	//	}
	//}

	ctx.Type = "json"
	ctx.Body = httpApiBody{code, text, data}
}

//通用方法
func (ctx *Http) UserAgent() string {
	return ctx.Header("User-Agent")
}
func (ctx *Http) Ip() string {
	ip := "127.0.0.1"

	req := ctx.req.Reader

	if forwarded := req.Header.Get("x-forwarded-for"); forwarded != "" {
		ip = forwarded
	} else if realIp := req.Header.Get("X-Real-IP"); realIp != "" {
		ip = realIp
	} else {
		newip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err == nil {
			ip = newip
		}
	}

	return ip
}
