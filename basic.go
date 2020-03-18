package ark

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	. "github.com/arkgo/asset"
)

type (
	basicConfig struct {
		State   string `toml:"state"`
		Mime    string `toml:"mime"`
		Regular string `toml:"regular"`
		Lang    string `toml:"lang"`
	}

	//语言配置
	langConfig struct {
		Name    string   `toml:"name"`
		Text    string   `toml:"text"`
		Accepts []string `toml:"accepts"`
	}

	basicModule struct {
		mutex sync.Mutex

		states   map[string]State
		langs    map[string]string
		mimes    map[string]Mime
		regulars map[string]Regular
		types    map[string]Type
		cryptos  map[string]Crypto
	}

	State struct {
		Name   string   `json:"name"`
		Desc   string   `json:"desc"`
		Alias  []string `json:"alias"`
		Code   int      `json:"code"`
		String string   `json:"string"`
	}
	Mime struct {
		Name  string   `json:"name"`
		Desc  string   `json:"desc"`
		Alias []string `json:"alias"`
		Type  string   `json:"-"`
		Types []string `json:"types"`
	}
	Regular struct {
		Name        string   `json:"name"`
		Desc        string   `json:"desc"`
		Alias       []string `json:"alias"`
		Expression  string   `json:"-"`
		Expressions []string `json:"expressions"`
	}

	TypeValidFunc func(Any, Var) bool
	TypeValueFunc func(Any, Var) Any
	Type          struct {
		Name    string        `json:"name"`
		Desc    string        `json:"desc"`
		Alias   []string      `json:"alias"`
		Setting Map           `json:"setting"`
		Valid   TypeValidFunc `json:"-"`
		Value   TypeValueFunc `json:"-"`
	}

	CryptoEncodeFunc func(Any, Var) Any
	CryptoDecodeFunc func(Any, Var) Any
	Crypto           struct {
		Name    string           `json:"name"`
		Desc    string           `json:"desc"`
		Alias   []string         `json:"alias"`
		Setting Map              `json:"setting"`
		Encode  CryptoEncodeFunc `json:"-"`
		Decode  CryptoDecodeFunc `json:"-"`
	}
)

func newBasic() *basicModule {

	basic := &basicModule{
		states:   make(map[string]State, 0),
		mimes:    make(map[string]Mime, 0),
		regulars: make(map[string]Regular, 0),
		langs:    make(map[string]string, 0),
		types:    make(map[string]Type, 0),
		cryptos:  make(map[string]Crypto, 0),
	}

	//这里加载语言文件，和其它定义

	//加载状态定义
	//状态定义不需要动态处理，写死在代码里就完事了
	// var states map[string]int
	// err := loading(path.Join(ark.Config.Basic.State), states)
	// if err == nil {
	// 	for key, val := range states {
	// 		basic.State(key, State{Code: val})
	// 	}
	// }

	//加载mime类型
	var mimes map[string][]string
	err := loading(path.Join(ark.Config.Basic.Mime), mimes)
	if err == nil {
		for key, vals := range mimes {
			basic.Mime(key, Mime{Types: vals})
		}
	}

	//加载正则表达式
	var regulars map[string][]string
	err = loading(path.Join(ark.Config.Basic.Regular), regulars)
	if err == nil {
		for key, vals := range regulars {
			basic.Regular(key, Regular{Expressions: vals})
		}
	}

	//加载语言包
	var langs map[string]string
	err = loading(path.Join(ark.Config.Basic.Lang, DEFAULT+".toml"), langs)
	if err == nil {
		basic.Lang(DEFAULT, langs)
	}
	for lang, _ := range ark.Config.Lang {
		var langs map[string]string
		err := loading(path.Join(ark.Config.Basic.Lang, lang+".toml"), langs)
		if err == nil {
			basic.Lang(lang, langs)
		}
	}

	return basic
}

// func (module *basicModule) State(config map[string]int, overrides ...bool) {
// 	module.mutex.Lock()
// 	defer module.mutex.Unlock()

// 	override := true
// 	if len(overrides) > 0 {
// 		override = overrides[0]
// 	}

// 	for k, v := range config {
// 		if override {
// 			module.states[k] = v
// 		} else {
// 			if _, ok := module.states[k]; ok == false {
// 				module.states[k] = v
// 			}
// 		}
// 	}
// }

func (module *basicModule) State(name string, config State, overrides ...bool) {
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
		module.mutex.Lock()
		if override {
			module.states[key] = config
		} else {
			if _, ok := module.states[key]; ok == false {
				module.states[key] = config
			}
		}
		module.mutex.Unlock()

		//自动注册默认的语言字串
		if config.String != "" {
			module.Lang(DEFAULT, map[string]string{key: config.String})
		}
	}
}

func (module *basicModule) Code(state string, defs ...int) int {
	if state, ok := module.states[state]; ok {
		return state.Code
	}
	if len(defs) > 0 {
		return defs[0]
	}
	return -1
}

func (module *basicModule) Results(langs ...string) map[int]string {
	lang := DEFAULT
	if len(langs) > 0 {
		lang = langs[0]
	}

	codes := map[int]string{}
	for key, state := range module.states {
		codes[state.Code] = module.String(lang, key)
	}
	return codes
}

func (module *basicModule) Mime(name string, config Mime, overrides ...bool) {
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

	if config.Types == nil {
		config.Types = []string{}
	}
	if config.Type != "" {
		config.Types = append(config.Types, config.Type)
	}

	for _, key := range alias {
		if override {
			module.mimes[key] = config
		} else {
			if _, ok := module.mimes[key]; ok == false {
				module.mimes[key] = config
			}
		}
	}
}

// func (module *basicModule) Mime(config map[string][]string, overrides ...bool) {
// 	module.mutex.Lock()
// 	defer module.mutex.Unlock()

// 	override := true
// 	if len(overrides) > 0 {
// 		override = overrides[0]
// 	}

// 	for k, v := range config {
// 		if override {
// 			module.mimes[k] = v
// 		} else {
// 			if _, ok := module.mimes[k]; ok == false {
// 				module.mimes[k] = v
// 			}
// 		}
// 	}
// }

//按mime拿类型（扩展名）
func (module *basicModule) Extension(mime string, defs ...string) string {
	if strings.Contains(mime, "/") == false {
		return mime
	}
	for ext, config := range module.mimes {
		for _, v := range config.Types {
			if strings.ToLower(v) == strings.ToLower(mime) {
				return ext
			}
		}
	}
	if len(defs) > 0 {
		return defs[0]
	}
	return ""
}

//按扩展名拿mime
func (module *basicModule) Mimetype(ext string, defs ...string) string {

	if strings.Contains(ext, "/") {
		return ext
	}

	if strings.HasPrefix(ext, ".") {
		ext = strings.TrimPrefix(ext, ".")
	}

	if vs, ok := module.mimes[ext]; ok && len(vs.Types) > 0 {
		return vs.Types[0]
	}
	if vs, ok := module.mimes["*"]; ok && len(vs.Types) > 0 {
		return vs.Types[0]
	}
	if len(defs) > 0 {
		return defs[0]
	}

	return "application/octet-stream"
}

// func (module *basicModule) Regular(config map[string][]string, overrides ...bool) {
// 	module.mutex.Lock()
// 	defer module.mutex.Unlock()

// 	override := true
// 	if len(overrides) > 0 {
// 		override = overrides[0]
// 	}

// 	for k, v := range config {
// 		if override {
// 			module.regulars[k] = v
// 		} else {
// 			if _, ok := module.regulars[k]; ok == false {
// 				module.regulars[k] = v
// 			}
// 		}
// 	}
// }

func (module *basicModule) Regular(name string, config Regular, overrides ...bool) {
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

	if config.Expressions == nil {
		config.Expressions = []string{}
	}
	if config.Expression != "" {
		config.Expressions = append(config.Expressions, config.Expression)
	}

	for _, key := range alias {
		if override {
			module.regulars[key] = config
		} else {
			if _, ok := module.regulars[key]; ok == false {
				module.regulars[key] = config
			}
		}
	}
}
func (module *basicModule) Expressions(name string, defs ...string) []string {
	if config, ok := module.regulars[name]; ok {
		return config.Expressions
	}
	return defs
}

func (module *basicModule) Match(value, regular string) bool {
	matchs := module.Expressions(regular)
	for _, v := range matchs {
		regx := regexp.MustCompile(v)
		if regx.MatchString(value) {
			return true
		}
	}
	return false
}

//lang做为前缀，加.和key分开
func (module *basicModule) Lang(lang string, config map[string]string, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	override := true
	if len(overrides) > 0 {
		override = overrides[0]
	}

	for k, v := range config {
		key := fmt.Sprintf("%v.%v", lang, k)
		if override {
			module.langs[key] = v
		} else {
			if _, ok := module.langs[k]; ok == false {
				module.langs[key] = v
			}
		}
	}
}

func (module *basicModule) String(lang, name string, args ...Any) string {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if lang == "" {
		lang = DEFAULT
	}

	defaultKey := fmt.Sprintf("%v.%v", DEFAULT, name)
	langKey := fmt.Sprintf("%v.%v", lang, name)

	langStr := ""

	if vv, ok := module.langs[langKey]; ok && vv != "" {
		langStr = vv
	} else if vv, ok := module.langs[defaultKey]; ok && vv != "" {
		langStr = vv
	} else {
		langStr = name
	}

	if len(args) > 0 {
		ccc := strings.Count(langStr, "%") - strings.Count(langStr, "%%")
		if ccc == len(args) {
			return fmt.Sprintf(langStr, args...)
		}
	}
	return langStr
}

func (module *basicModule) Type(name string, config Type, overrides ...bool) {
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
			module.types[key] = config
		} else {
			if _, ok := module.types[key]; ok == false {
				module.types[key] = config
			}
		}
	}

}

func (module *basicModule) Types() map[string]Type {
	types := map[string]Type{}
	for k, v := range module.types {
		types[k] = v
	}
	return types
}

func (module *basicModule) Cryptos() map[string]Crypto {
	cryptos := map[string]Crypto{}
	for k, v := range module.cryptos {
		cryptos[k] = v
	}
	return cryptos
}
func (module *basicModule) Crypto(name string, config Crypto, overrides ...bool) {
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
			module.cryptos[key] = config
		} else {
			if _, ok := module.cryptos[key]; ok == false {
				module.cryptos[key] = config
			}
		}
	}
}

func (module *basicModule) typeDefaultValid(value Any, config Var) bool {
	return module.Match(fmt.Sprintf("%s", value), config.Type)
}
func (module *basicModule) typeDefaultValue(value Any, config Var) Any {
	return fmt.Sprintf("%s", value)
}

func (module *basicModule) typeValid(name string) TypeValidFunc {
	if config, ok := module.types[name]; ok {
		if config.Valid != nil {
			return config.Valid
		}
	}
	return module.typeDefaultValid
}
func (module *basicModule) typeValue(name string) TypeValueFunc {
	if config, ok := module.types[name]; ok {
		if config.Value != nil {
			return config.Value
		}
	}
	return module.typeDefaultValue
}
func (module *basicModule) typeMethod(name string) (TypeValidFunc, TypeValueFunc) {
	return module.typeValid(name), module.typeValue(name)
}

func (module *basicModule) cryptoDefaultEncode(value Any, config Var) Any {
	return value
}
func (module *basicModule) cryptoDefaultDecode(value Any, config Var) Any {
	return value
}

func (module *basicModule) cryptoEncode(name string) CryptoEncodeFunc {
	if config, ok := module.cryptos[name]; ok {
		if config.Encode != nil {
			return config.Encode
		}
	}
	return module.cryptoDefaultEncode
}
func (module *basicModule) cryptoDecode(name string) CryptoDecodeFunc {
	if config, ok := module.cryptos[name]; ok {
		if config.Decode != nil {
			return config.Decode
		}
	}
	return module.cryptoDefaultDecode
}

// func (module *basicModule) cryptoMethod(name string) (CryptoEncodeFunc, CryptoDecodeFunc) {
// 	return module.cryptoEncode(name), module.cryptoDecode(name)
// }

//一定要返回*Result，
//因为在定义参数的时候，可以自定义empty,error两个属性， 返回自定义的结果
// func (module *basicModule) Mapping(config Map, data Map, value Map, argn bool, pass bool, ctxs ...context) *Res {
// 	var ctx context
// 	if len(ctxs) > 0 {
// 		ctx = ctxs[0]
// 	}

// 	/*
// 	   argn := false
// 	   if len(args) > 0 {
// 	       argn = args[0]
// 	   }
// 	*/

// 	//遍历配置	begin
// 	for fieldName, fv := range config {

// 		fieldConfig := Map{}

// 		//注意，这里存在2种情况
// 		//1. Map对象
// 		//2. map[string]interface{}
// 		//要分开处理
// 		//go1.9以后可以 type xx=yy 就只要处理一个了

// 		switch c := fv.(type) {
// 		case Map:
// 			fieldConfig = c
// 		default:
// 			//类型不对，跳过
// 			continue
// 		}

// 		//解过密？
// 		decoded := false
// 		passEmpty := false
// 		passError := false

// 		//Map 如果是JSON文件，或是发过来的消息，就不支持
// 		//而用下面的，就算是MAP也可以支持，OK
// 		//麻烦来了，web.args用下面这样处理不了
// 		//if fieldConfig, ok := fv.(map[string]interface{}); ok {

// 		fieldMust, fieldEmpty := false, false
// 		fieldValue, fieldExist := data[fieldName]
// 		fieldAuto, fieldJson := fieldConfig["auto"], fieldConfig["json"]
// 		//_, fieldEmpty = data[fieldName]

// 		/* 处理是否必填和为空 */
// 		if v, ok := fieldConfig["must"]; ok {
// 			if v == nil {
// 				fieldEmpty = true
// 			}
// 			if vv, ok := v.(bool); ok {
// 				fieldMust = vv
// 			}
// 		}

// 		//trees := append(tree, fieldName)
// 		//fmt.Printf("trees=%v". strings.Join(trees, "."))

// 		//fmt.Printf("t=%s, argn=%v, value=%v\n", strings.Join(trees, "."), argn, fieldValue)
// 		//fmt.Printf("trees=%v, must=%v, empty=%v, exist=%v, auto=%v, value=%v, config=%v\n\n",
// 		//	strings.Join(trees, "."), fieldMust, fieldEmpty, fieldExist, fieldAuto, fieldValue, fieldConfig)

// 		//fmt.Println("mapping", fieldName)

// 		strVal := fmt.Sprintf("%v", fieldValue)

// 		//等一下。 空的map[]无字段。 需要也表示为空吗?
// 		//if strVal == "" || strVal == "map[]" || strVal == "{}"{

// 		//因为go1.6之后。把一个值为nil的map  再写入map之后, 判断 if map[key]==nil 就无效了
// 		if strVal == "" || data[fieldName] == nil || (fieldMust != true && strVal == "map[]") {
// 			fieldValue = nil
// 		}

// 		//如果不可为空，但是为空了，
// 		if fieldMust && fieldEmpty == false && (fieldValue == nil || strVal == "") && fieldAuto == nil && fieldJson == nil && argn == false {

// 			//是否跳过
// 			if pass {
// 				passEmpty = true
// 			} else {
// 				//是否有自定义的状态
// 				if empty, ok := fieldConfig["empty"].(string); ok && empty != "" {
// 					//自定义的状态下， 应该不用把参数名传过去了，都自定义了
// 					return newResult(empty)
// 				} else if empty, ok := fieldConfig["empty"].(*Res); ok {
// 					return empty
// 				} else {
// 					//这样方便在多语言环境使用
// 					key := ".mapping.empty." + fieldName
// 					if module.Code(key, -999) == -999 {
// 						return newResult(".mapping.empty", fieldConfig["name"])
// 					}
// 					return newResult(key)
// 				}
// 			}

// 		} else {

// 			//如果值为空的时候，看有没有默认值
// 			//到这里值是可以为空的
// 			if fieldValue == nil || strVal == "" {

// 				//如果有默认值
// 				//可为NULL时，不给默认值
// 				if fieldAuto != nil && !argn {

// 					//暂时不处理 $id, $date 之类的定义好的默认值，不包装了
// 					switch autoValue := fieldAuto.(type) {
// 					case func() interface{}:
// 						fieldValue = autoValue()
// 					case func() time.Time:
// 						fieldValue = autoValue()
// 						//case func() bson.ObjectId:	//待处理
// 						//fieldValue = autoValue()
// 					case func() string:
// 						fieldValue = autoValue()
// 					case func() int:
// 						fieldValue = int64(autoValue())
// 					case func() int8:
// 						fieldValue = int64(autoValue())
// 					case func() int16:
// 						fieldValue = int64(autoValue())
// 					case func() int32:
// 						fieldValue = int64(autoValue())
// 					case func() int64:
// 						fieldValue = autoValue()
// 					case func() float32:
// 						fieldValue = float64(autoValue())
// 					case func() float64:
// 						fieldValue = autoValue()
// 					case int:
// 						{
// 							fieldValue = int64(autoValue)
// 						}
// 					case int8:
// 						{
// 							fieldValue = int64(autoValue)
// 						}
// 					case int16:
// 						{
// 							fieldValue = int64(autoValue)
// 						}
// 					case int32:
// 						{
// 							fieldValue = int64(autoValue)
// 						}
// 					case float32:
// 						{
// 							fieldValue = float64(autoValue)
// 						}

// 					case []int:
// 						{
// 							arr := []int64{}
// 							for _, nv := range autoValue {
// 								arr = append(arr, int64(nv))
// 							}
// 							fieldValue = arr
// 						}
// 					case []int8:
// 						{
// 							arr := []int64{}
// 							for _, nv := range autoValue {
// 								arr = append(arr, int64(nv))
// 							}
// 							fieldValue = arr
// 						}
// 					case []int16:
// 						{
// 							arr := []int64{}
// 							for _, nv := range autoValue {
// 								arr = append(arr, int64(nv))
// 							}
// 							fieldValue = arr
// 						}
// 					case []int32:
// 						{
// 							arr := []int64{}
// 							for _, nv := range autoValue {
// 								arr = append(arr, int64(nv))
// 							}
// 							fieldValue = arr
// 						}

// 					case []float32:
// 						{
// 							arr := []float64{}
// 							for _, nv := range autoValue {
// 								arr = append(arr, float64(nv))
// 							}
// 							fieldValue = arr
// 						}

// 					default:
// 						fieldValue = autoValue
// 					}

// 					//默认值是不是也要包装一下，这里只包装值，不做验证
// 					if fieldType, ok := fieldConfig["type"].(string); ok {
// 						_, fieldValueCall := module.typeMethod(fieldType)

// 						//如果配置中有自己的值函数
// 						if f, ok := fieldConfig["value"].(func(Any, Map) Any); ok {
// 							fieldValueCall = f
// 						}

// 						//包装值
// 						if fieldValueCall != nil {
// 							fieldValue = fieldValueCall(fieldValue, fieldConfig)
// 						}
// 					}

// 				} else { //没有默认值, 且值为空

// 					//有个问题, POST表单的时候.  表单字段如果有，值是存在的，会是空字串
// 					//但是POST的时候如果有argn, 实际上是不想存在此字段的

// 					//如果字段可以不存在
// 					if fieldEmpty || argn {

// 						//当empty(argn)=true，并且如果传过值中存在字段的KEY，值就要存在，以更新为null
// 						if argn && fieldExist {
// 							//不操作，自然往下执行
// 						} else { //值可以不存在
// 							continue
// 						}

// 					} else if argn {

// 					} else {
// 						//到这里不管
// 						//因为字段必须存在，但是值为空
// 					}
// 				}

// 			} else { //值不为空，处理值

// 				//值处理前，是不是需要解密
// 				//如果解密哦
// 				//decode
// 				if ct, ok := fieldConfig["decode"].(string); ok && ct != "" {

// 					//有一个小bug这里，在route的时候， 如果传的是string，本来是想加密的， 结果这里变成了解密
// 					//还得想个办法解决这个问题，所以，在传值的时候要注意，另外string型加密就有点蛋疼了
// 					//主要是在route的时候，其它的时候还ok，所以要在encode/decode中做类型判断解还是不解

// 					//而且要值是string类型
// 					// if sv,ok := fieldValue.(string); ok {

// 					//得到解密方法
// 					decode, setting := module.cryptoDecode(ct)
// 					if val := decode(fieldValue, setting); val != nil {
// 						//前方解过密了，表示该参数，不再加密
// 						//因为加密解密，只有一个2选1的
// 						//比如 args 只需要解密 data 只需要加密
// 						//route 的时候 args 需要加密，而不用再解，所以是单次的
// 						fieldValue = val
// 						decoded = true
// 					}
// 					// }
// 				}

// 				//验证放外面来，因为默认值也要验证和包装

// 				//按类型来做处理

// 				//验证方法和值方法
// 				//但是因为默认值的情况下，值有可能是为空的，所以要判断多一点
// 				if fieldType, ok := fieldConfig["type"].(string); ok && fieldType != "" {
// 					fieldValidCall, fieldValueCall := module.typeMethod(fieldType)

// 					//如果配置中有自己的验证函数
// 					if f, ok := fieldConfig["valid"].(func(Any, Map) bool); ok {
// 						fieldValidCall = f
// 					}
// 					//如果配置中有自己的值函数
// 					if f, ok := fieldConfig["value"].(func(Any, Map) Any); ok {
// 						fieldValueCall = f
// 					}

// 					//开始调用验证
// 					if fieldValidCall != nil {
// 						//如果验证通过
// 						if fieldValidCall(fieldValue, fieldConfig) {
// 							//包装值
// 							if fieldValueCall != nil {
// 								//对时间值做时区处理
// 								if ctx != nil && ctx.Zone() != time.Local {
// 									if vv, ok := fieldValue.(time.Time); ok {
// 										fieldValue = vv.In(ctx.Zone())
// 									} else if vvs, ok := fieldValue.([]time.Time); ok {
// 										newTimes := []time.Time{}
// 										for _, vv := range vvs {
// 											newTimes = append(newTimes, vv.In(ctx.Zone()))
// 										}
// 										fieldValue = newTimes
// 									}
// 								}

// 								fieldValue = fieldValueCall(fieldValue, fieldConfig)
// 							}
// 						} else { //验证不通过

// 							//是否可以跳过
// 							if pass {
// 								passError = true
// 							} else {

// 								//是否有自定义的状态
// 								if c, ok := fieldConfig["error"].(string); ok && c != "" {
// 									//自定义的状态下， 应该不用把参数名传过去了，都自定义了
// 									return newResult(c)
// 								} else if result, ok := fieldConfig["error"].(*Res); ok {
// 									return result
// 								} else {
// 									//这样方便在多语言环境使用
// 									key := ".mapping.error." + fieldName
// 									if module.Code(key, -999) == -999 {
// 										return newResult(".mapping.error", fieldConfig["name"])
// 									}
// 									return newResult(key)
// 								}
// 							}
// 						}
// 					}
// 				}

// 			}

// 		}

// 		//这后面是总的字段处理
// 		//如JSON，加密

// 		//如果是JSON， 或是数组啥的处理
// 		//注意，当 json 本身可为空，下级有不可为空的，值为空时， 应该跳过子级的检查
// 		//如果 json 可为空， 就不应该有 默认值， 定义的时候要注意啊啊啊啊
// 		//理论上，只要JSON可为空～就不处理下一级json
// 		jsonning := true
// 		if !fieldMust && fieldValue == nil {
// 			jsonning = false
// 		}

// 		//还有种情况要处理. 当type=json, must=true的时候,有默认值, 但是没有定义json节点.

// 		if m, ok := fieldConfig["json"]; ok && jsonning {
// 			jsonConfig := Map{}
// 			//注意，这里存在2种情况
// 			//1. Map对象 //2. map[string]interface{}
// 			switch c := m.(type) {
// 			case Map:
// 				jsonConfig = c
// 			}

// 			//如果是数组
// 			isArray := false
// 			//fieldData到这里定义
// 			fieldData := []Map{}

// 			switch v := fieldValue.(type) {
// 			case Map:
// 				fieldData = append(fieldData, v)
// 			case []Map:
// 				isArray = true
// 				fieldData = v
// 			default:
// 				fieldData = []Map{}
// 			}

// 			//直接都遍历
// 			values := []Map{}

// 			for _, d := range fieldData {
// 				v := Map{}

// 				// err := module.Parse(trees, jsonConfig, d, v, argn, pass);
// 				err := module.Mapping(jsonConfig, d, v, argn, pass, ctx)
// 				if err != nil {
// 					return err
// 				} else {
// 					//fieldValue = append(fieldValue, v)
// 					values = append(values, v)
// 				}
// 			}

// 			if isArray {
// 				fieldValue = values
// 			} else {
// 				if len(values) > 0 {
// 					fieldValue = values[0]
// 				} else {
// 					fieldValue = Map{}
// 				}
// 			}

// 		}

// 		// 跳过且为空时，不写值
// 		if pass && (passEmpty || passError) {
// 		} else {

// 			//当pass=true的时候， 这里可能会是空值，那应该跳过
// 			//最后，值要不要加密什么的
// 			//如果加密
// 			//encode
// 			if ct, ok := fieldConfig["encode"].(string); ok && decoded == false && passEmpty == false && passError == false {

// 				/*
// 				   //全都转成字串再加密
// 				   //为什么要全部转字串才能加密？
// 				   //不用转了，因为hashid这样的加密就要int64
// 				*/

// 				encode, setting := module.cryptoEncode(ct)
// 				if val := encode(fieldValue, setting); val != nil {
// 					fieldValue = val
// 				}
// 			}
// 		}

// 		//没有JSON要处理，所以给值
// 		value[fieldName] = fieldValue

// 	}

// 	return nil
// 	//遍历配置	end
// }

func (module *basicModule) Mapping(config Vars, data Map, value Map, argn bool, pass bool, ctxs ...context) *Res {
	var ctx context
	if len(ctxs) > 0 {
		ctx = ctxs[0]
	}

	/*
	   argn := false
	   if len(args) > 0 {
	       argn = args[0]
	   }
	*/

	//遍历配置	begin
	for fieldName, fieldConfig := range config {

		//注意，这里存在2种情况
		//1. Map对象
		//2. map[string]interface{}
		//要分开处理
		//go1.9以后可以 type xx=yy 就只要处理一个了

		// switch c := fv.(type) {
		// case Map:
		// 	fieldConfig = c
		// default:
		// 	//类型不对，跳过
		// 	continue
		// }

		//解过密？
		decoded := false
		passEmpty := false
		passError := false

		//Map 如果是JSON文件，或是发过来的消息，就不支持
		//而用下面的，就算是MAP也可以支持，OK
		//麻烦来了，web.args用下面这样处理不了
		//if fieldConfig, ok := fv.(map[string]interface{}); ok {

		fieldMust, fieldEmpty := fieldConfig.Require, fieldConfig.Nullable
		fieldValue, fieldExist := data[fieldName]
		fieldAuto, fieldJson := fieldConfig.Default, fieldConfig.Children
		//_, fieldEmpty = data[fieldName]

		//trees := append(tree, fieldName)
		//fmt.Printf("trees=%v". strings.Join(trees, "."))

		//fmt.Printf("t=%s, argn=%v, value=%v\n", strings.Join(trees, "."), argn, fieldValue)
		//fmt.Printf("trees=%v, must=%v, empty=%v, exist=%v, auto=%v, value=%v, config=%v\n\n",
		//	strings.Join(trees, "."), fieldMust, fieldEmpty, fieldExist, fieldAuto, fieldValue, fieldConfig)

		//fmt.Println("mapping", fieldName)

		strVal := fmt.Sprintf("%v", fieldValue)

		//等一下。 空的map[]无字段。 需要也表示为空吗?
		//if strVal == "" || strVal == "map[]" || strVal == "{}"{

		//因为go1.6之后。把一个值为nil的map  再写入map之后, 判断 if map[key]==nil 就无效了
		if strVal == "" || data[fieldName] == nil || (fieldMust != true && strVal == "map[]") {
			fieldValue = nil
		}

		//如果不可为空，但是为空了，
		if fieldMust && fieldEmpty == false && (fieldValue == nil || strVal == "") && fieldAuto == nil && fieldJson == nil && argn == false {

			//是否跳过
			if pass {
				passEmpty = true
			} else {
				//是否有自定义的状态
				if fieldConfig.Empty != nil {
					return fieldConfig.Empty
				} else {
					//这样方便在多语言环境使用
					key := "_mapping_empty_" + fieldName
					if module.Code(key, -999) == -999 {
						return newResult("_mapping_empty", fieldConfig.Name)
					}
					return newResult(key)
				}
			}

		} else {

			//如果值为空的时候，看有没有默认值
			//到这里值是可以为空的
			if fieldValue == nil || strVal == "" {

				//如果有默认值
				//可为NULL时，不给默认值
				if fieldAuto != nil && !argn {

					//暂时不处理 $id, $date 之类的定义好的默认值，不包装了
					switch autoValue := fieldAuto.(type) {
					case func() interface{}:
						fieldValue = autoValue()
					case func() time.Time:
						fieldValue = autoValue()
						//case func() bson.ObjectId:	//待处理
						//fieldValue = autoValue()
					case func() string:
						fieldValue = autoValue()
					case func() int:
						fieldValue = int64(autoValue())
					case func() int8:
						fieldValue = int64(autoValue())
					case func() int16:
						fieldValue = int64(autoValue())
					case func() int32:
						fieldValue = int64(autoValue())
					case func() int64:
						fieldValue = autoValue()
					case func() float32:
						fieldValue = float64(autoValue())
					case func() float64:
						fieldValue = autoValue()
					case int:
						{
							fieldValue = int64(autoValue)
						}
					case int8:
						{
							fieldValue = int64(autoValue)
						}
					case int16:
						{
							fieldValue = int64(autoValue)
						}
					case int32:
						{
							fieldValue = int64(autoValue)
						}
					case float32:
						{
							fieldValue = float64(autoValue)
						}

					case []int:
						{
							arr := []int64{}
							for _, nv := range autoValue {
								arr = append(arr, int64(nv))
							}
							fieldValue = arr
						}
					case []int8:
						{
							arr := []int64{}
							for _, nv := range autoValue {
								arr = append(arr, int64(nv))
							}
							fieldValue = arr
						}
					case []int16:
						{
							arr := []int64{}
							for _, nv := range autoValue {
								arr = append(arr, int64(nv))
							}
							fieldValue = arr
						}
					case []int32:
						{
							arr := []int64{}
							for _, nv := range autoValue {
								arr = append(arr, int64(nv))
							}
							fieldValue = arr
						}

					case []float32:
						{
							arr := []float64{}
							for _, nv := range autoValue {
								arr = append(arr, float64(nv))
							}
							fieldValue = arr
						}

					default:
						fieldValue = autoValue
					}

					//默认值是不是也要包装一下，这里只包装值，不做验证
					if fieldConfig.Type != "" {
						_, fieldValueCall := module.typeMethod(fieldConfig.Type)

						//如果配置中有自己的值函数
						if fieldConfig.Value != nil {
							fieldValueCall = fieldConfig.Value
						}

						//包装值
						if fieldValueCall != nil {
							//待处理：到底是传config还是setting
							fieldValue = fieldValueCall(fieldValue, fieldConfig)
						}
					}

				} else { //没有默认值, 且值为空

					//有个问题, POST表单的时候.  表单字段如果有，值是存在的，会是空字串
					//但是POST的时候如果有argn, 实际上是不想存在此字段的

					//如果字段可以不存在
					if fieldEmpty || argn {

						//当empty(argn)=true，并且如果传过值中存在字段的KEY，值就要存在，以更新为null
						if argn && fieldExist {
							//不操作，自然往下执行
						} else { //值可以不存在
							continue
						}

					} else if argn {

					} else {
						//到这里不管
						//因为字段必须存在，但是值为空
					}
				}

			} else { //值不为空，处理值

				//值处理前，是不是需要解密
				//如果解密哦
				//decode
				if fieldConfig.Decode != "" {

					//有一个小bug这里，在route的时候， 如果传的是string，本来是想加密的， 结果这里变成了解密
					//还得想个办法解决这个问题，所以，在传值的时候要注意，另外string型加密就有点蛋疼了
					//主要是在route的时候，其它的时候还ok，所以要在encode/decode中做类型判断解还是不解

					//而且要值是string类型
					// if sv,ok := fieldValue.(string); ok {

					//得到解密方法
					decode := module.cryptoDecode(fieldConfig.Decode)
					if val := decode(fieldValue, fieldConfig); val != nil {
						//前方解过密了，表示该参数，不再加密
						//因为加密解密，只有一个2选1的
						//比如 args 只需要解密 data 只需要加密
						//route 的时候 args 需要加密，而不用再解，所以是单次的
						fieldValue = val
						decoded = true
					}
					// }
				}

				//验证放外面来，因为默认值也要验证和包装

				//按类型来做处理

				//验证方法和值方法
				//但是因为默认值的情况下，值有可能是为空的，所以要判断多一点
				if fieldConfig.Type != "" {
					fieldValidCall, fieldValueCall := module.typeMethod(fieldConfig.Type)

					//如果配置中有自己的验证函数
					if fieldConfig.Valid != nil {
						fieldValidCall = fieldConfig.Valid
					}
					//如果配置中有自己的值函数
					if fieldConfig.Value != nil {
						fieldValueCall = fieldConfig.Value
					}

					//开始调用验证
					if fieldValidCall != nil {
						//如果验证通过
						//待处理：这里到底是传setting还是fieldconfig比较好
						if fieldValidCall(fieldValue, fieldConfig) {
							//包装值
							if fieldValueCall != nil {
								//对时间值做时区处理
								if ctx != nil && ctx.Zone() != time.Local {
									if vv, ok := fieldValue.(time.Time); ok {
										fieldValue = vv.In(ctx.Zone())
									} else if vvs, ok := fieldValue.([]time.Time); ok {
										newTimes := []time.Time{}
										for _, vv := range vvs {
											newTimes = append(newTimes, vv.In(ctx.Zone()))
										}
										fieldValue = newTimes
									}
								}

								//待处理：这里到底是传setting还是config
								fieldValue = fieldValueCall(fieldValue, fieldConfig)
							}
						} else { //验证不通过

							//是否可以跳过
							if pass {
								passError = true
							} else {

								//是否有自定义的状态
								if fieldConfig.Error != nil {
									return fieldConfig.Error
								} else {
									//这样方便在多语言环境使用
									key := "_mapping_error_" + fieldName
									if module.Code(key, -999) == -999 {
										return newResult("_mapping_error", fieldConfig.Name)
									}
									return newResult(key)
								}
							}
						}
					}
				}

			}

		}

		//这后面是总的字段处理
		//如JSON，加密

		//如果是JSON， 或是数组啥的处理
		//注意，当 json 本身可为空，下级有不可为空的，值为空时， 应该跳过子级的检查
		//如果 json 可为空， 就不应该有 默认值， 定义的时候要注意啊啊啊啊
		//理论上，只要JSON可为空～就不处理下一级json
		jsonning := true
		if !fieldMust && fieldValue == nil {
			jsonning = false
		}

		//还有种情况要处理. 当type=json, must=true的时候,有默认值, 但是没有定义json节点.

		if fieldConfig.Children != nil && jsonning {
			jsonConfig := fieldConfig.Children
			//注意，这里存在2种情况
			//1. Map对象 //2. map[string]interface{}

			// switch c := m.(type) {
			// case Map:
			// 	jsonConfig = c
			// }

			//如果是数组
			isArray := false
			//fieldData到这里定义
			fieldData := []Map{}

			switch v := fieldValue.(type) {
			case Map:
				fieldData = append(fieldData, v)
			case []Map:
				isArray = true
				fieldData = v
			default:
				fieldData = []Map{}
			}

			//直接都遍历
			values := []Map{}

			for _, d := range fieldData {
				v := Map{}

				// err := module.Parse(trees, jsonConfig, d, v, argn, pass);
				err := module.Mapping(jsonConfig, d, v, argn, pass, ctx)
				if err != nil {
					return err
				} else {
					//fieldValue = append(fieldValue, v)
					values = append(values, v)
				}
			}

			if isArray {
				fieldValue = values
			} else {
				if len(values) > 0 {
					fieldValue = values[0]
				} else {
					fieldValue = Map{}
				}
			}

		}

		// 跳过且为空时，不写值
		if pass && (passEmpty || passError) {
		} else {

			//当pass=true的时候， 这里可能会是空值，那应该跳过
			//最后，值要不要加密什么的
			//如果加密
			//encode
			if fieldConfig.Encode != "" && decoded == false && passEmpty == false && passError == false {

				/*
				   //全都转成字串再加密
				   //为什么要全部转字串才能加密？
				   //不用转了，因为hashid这样的加密就要int64
				*/

				encode := module.cryptoEncode(fieldConfig.Encode)
				if val := encode(fieldValue, fieldConfig); val != nil {
					fieldValue = val
				}
			}
		}

		//没有JSON要处理，所以给值
		value[fieldName] = fieldValue

	}

	return nil
	//遍历配置	end
}

// func State(config Map, overrides ...bool) {
// 	ms := map[string]int{}

// 	for k, v := range config {
// 		if vv, ok := v.(int); ok {
// 			ms[k] = vv
// 		} else if vv, ok := v.(int64); ok {
// 			ms[k] = int(vv)
// 		}
// 	}

// 	ark.Basic.State(ms, overrides...)
// }
func Code(name string, defs ...int) int {
	return ark.Basic.Code(name, defs...)
}

// func Mime(config Map, overrides ...bool) {
// 	ms := map[string][]string{}

// 	for k, v := range config {
// 		switch vv := v.(type) {
// 		case string:
// 			ms[k] = []string{vv}
// 		case []string:
// 			ms[k] = vv
// 		}
// 	}

// 	ark.Basic.Mime(ms, overrides...)
// }
func Mimetype(ext string, defs ...string) string {
	return ark.Basic.Mimetype(ext, defs...)
}
func Extension(mime string, defs ...string) string {
	return ark.Basic.Extension(mime, defs...)
}
func String(lang, name string, args ...Any) string {
	return ark.Basic.String(lang, name, args...)
}

// func Regular(config Map, overrides ...bool) {
// 	rs := map[string][]string{}

// 	for k, v := range config {
// 		switch vv := v.(type) {
// 		case string:
// 			rs[k] = []string{vv}
// 		case []string:
// 			rs[k] = vv
// 		}
// 	}

// 	ark.Basic.Regular(rs, overrides...)
// }
// func Expressions(name string, defs ...string) []string {
// 	return ark.Basic.Expressions(name, defs...)
// }
func Match(value, regular string) bool {
	return ark.Basic.Match(value, regular)
}
func Results(langs ...string) map[int]string {
	return ark.Basic.Results(langs...)
}

//---------------------- mapping --------------------------
// func Type(name string, config Map, overrides ...bool) {
// 	ark.Basic.Type(name, config, overrides...)
// }
func Types() map[string]Type {
	return ark.Basic.Types()
}

// func Crypto(name string, config Map, overrides ...bool) {
// 	ark.Basic.Crypto(name, config, overrides...)
// }
func Cryptos() map[string]Crypto {
	return ark.Basic.Cryptos()
}
func Mapping(config Vars, data Map, value Map, argn bool, pass bool, ctxs ...context) *Res {
	return ark.Basic.Mapping(config, data, value, argn, pass, ctxs...)
}
