package ark

import (
	"time"

	. "github.com/arkgo/asset"
)

type (
	// context interface {
	// 	terminal()
	// 	Result(...*Res) *Res
	// 	Lang(...string) string
	// 	Zone(...*time.Location) *time.Location
	// 	dataBase(...string) DataBase
	// }

	context struct {
		lang      string
		zone      *time.Location
		lastError *Res
		databases map[string]DataBase
	}
)

func newcontext() *context {
	return &context{
		databases: make(map[string]DataBase),
		lang:      DEFAULT, zone: time.Local,
	}
}

// Lang 获取或设置当前上下文的语言
func (ctx *context) Lang(langs ...string) string {
	if ctx == nil {
		return DEFAULT
	}
	if len(langs) > 0 && langs[0] != "" {
		//待优化：加上配置中的语言判断，否则不修改
		ctx.lang = langs[0]
	}
	return ctx.lang
}

// Zone 获取或设置当前上下文的时区
func (ctx *context) Zone(zones ...*time.Location) *time.Location {
	if ctx == nil {
		return time.Local
	}

	if len(zones) > 0 && zones[0] != nil {
		ctx.zone = zones[0]
	}
	return ctx.zone
}

//最终的清理工作
func (ctx *context) terminal() {
	for _, base := range ctx.databases {
		base.Close()
	}
}

func (ctx *context) dataBase(bases ...string) DataBase {
	base := DEFAULT
	if len(bases) > 0 {
		base = bases[0]
	} else {
		for key := range ark.Data.connects {
			base = key
			break
		}
	}
	if _, ok := ctx.databases[base]; ok == false {
		ctx.databases[base] = ark.Data.Base(base)
	}
	return ctx.databases[base]
}

//返回最后的错误信息
//获取操作结果
func (ctx *context) Result(res ...*Res) *Res {
	if len(res) > 0 {
		err := res[0]
		ctx.lastError = err
		return err
	} else {
		err := ctx.lastError
		ctx.lastError = nil
		return err
	}
}

//获取langString
func (ctx *context) String(key string, args ...Any) string {
	return ark.Basic.String(ctx.Lang(), key, args...)
}

//----------------------- 签名系统 end ---------------------------------

// ------- 服务调用 -----------------
func (ctx *context) Invoke(name string, values ...Map) Map {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvv, res := ark.Service.Invoke(ctx, name, value)
	ctx.lastError = res
	return vvv
}

func (ctx *context) Invokes(name string, values ...Map) []Map {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvs, res := ark.Service.Invokes(ctx, name, value)
	ctx.lastError = res
	return vvs
}
func (ctx *context) Invoked(name string, values ...Map) bool {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvv, res := ark.Service.Invoked(ctx, name, value)
	ctx.lastError = res
	return vvv
}
func (ctx *context) Invoking(name string, offset, limit int64, values ...Map) (int64, []Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	count, items, res := ark.Service.Invoking(ctx, name, offset, limit, value)
	ctx.lastError = res
	return count, items
}

func (ctx *context) Invoker(name string, values ...Map) (Map, []Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	item, items, res := ark.Service.Invoker(ctx, name, value)
	ctx.lastError = res
	return item, items
}

func (ctx *context) Invokee(name string, values ...Map) float64 {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	count, res := ark.Service.Invokee(ctx, name, value)
	ctx.lastError = res
	return count
}

func (ctx *context) Logic(name string, settings ...Map) *Logic {
	return ark.Service.Logic(ctx, name, settings...)
}

//------- 服务调用 end-----------------

//语法糖
func (ctx *context) Locked(key string, expiry time.Duration, cons ...string) bool {
	return ark.Mutex.Lock(key, expiry, cons...) != nil
}
func (ctx *context) Lock(key string, expiry time.Duration, cons ...string) error {
	return ark.Mutex.Lock(key, expiry, cons...)
}
func (ctx *context) Unlock(key string, cons ...string) error {
	return ark.Mutex.Unlock(key, cons...)
}
