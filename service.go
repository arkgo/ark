package ark

import (
	"fmt"
	"sync"
	"time"

	"github.com/arkgo/asset"
	. "github.com/arkgo/asset"
)

type (
	serviceModule struct {
		mutex   sync.Mutex
		methods map[string]Method
	}

	serviceLibrary struct {
		module *serviceModule
		name   string
	}

	ServiceLogic struct {
		ctx     context
		Name    string
		Setting Map
	}

	Service struct {
		ctx     context
		Name    string
		Config  Method
		Setting Map
		Value   Map
		Args    Map
	}

	Method struct {
		Name     string   `json:"name"`
		Desc     string   `json:"desc"`
		Alias    []string `json:"alias"`
		Nullable bool     `json:"nullable"`
		Args     Vars     `json:"args"`
		Data     Vars     `json:"data"`
		Setting  Map      `json:"setting"`
		Action   Any      `json:"-"`
	}
)

func newService() *serviceModule {
	return &serviceModule{
		methods: make(map[string]Method, 0),
	}
}

//注册方法
func (module *serviceModule) Method(name string, config Method, overrides ...bool) {
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
			module.methods[key] = config
		} else {
			if _, ok := module.methods[key]; ok == false {
				module.methods[key] = config
			}
		}
	}
}

func (module *serviceModule) Invoke(ctx context, name string, value Map, settings ...Map) (Map, *Res) {
	if _, ok := module.methods[name]; ok == false {
		return nil, Fail
	}

	config := module.methods[name]

	var setting Map
	if len(settings) > 0 {
		setting = settings[0]
	}

	if ctx == nil {
		ctx = emptyContext()
		defer ctx.terminal()
	}
	if value == nil {
		value = make(Map)
	}
	if setting == nil {
		setting = make(Map)
	}

	args := Map{}
	if config.Args != nil {
		res := ark.Basic.Mapping(config.Args, value, args, config.Nullable, false, ctx)
		if res != nil {
			return nil, res
		}
	}

	service := &Service{
		ctx: ctx, Name: name, Config: config, Setting: setting,
		Value: value, Args: args,
	}

	data := Map{}
	var result *Res

	switch ff := config.Action.(type) {
	case func(*Service):
		ff(service)
	case func(*Service) *Res:
		result = ff(service)

	case func(*Service) bool:
		data = Map{
			"result": ff(service),
		}
	case func(*Service) Map:
		data = ff(service)
	case func(*Service) (Map, *Res):
		data, result = ff(service)
	case func(*Service) []Map:
		items := ff(service)
		data = Map{"items": items}
	case func(*Service) ([]Map, *Res):
		items, res := ff(service)
		data = Map{"items": items}
		result = res
	case func(*Service) (int64, []Map):
		count, items := ff(service)
		data = Map{"count": count, "items": items}
	case func(*Service) (Map, []Map):
		item, items := ff(service)
		data = Map{"item": item, "items": items}
	}

	//参数解析
	if config.Data != nil {
		out := Map{}
		err := ark.Basic.Mapping(config.Data, data, out, false, false, ctx)
		if err == nil {
			return out, result
		}
	}

	//参数如果解析失败，就原版返回
	return data, result
}
func (module *serviceModule) Invokes(ctx context, name string, value Map, settings ...Map) ([]Map, *Res) {
	data, res := module.Invoke(ctx, name, value, settings...)
	if res.Fail() {
		return []Map{}, res
	}
	if results, ok := data["items"].([]Map); ok {
		return results, res
	}
	return []Map{data}, res
}
func (module *serviceModule) Invoked(ctx context, name string, value Map, settings ...Map) (bool, *Res) {
	_, res := module.Invoke(ctx, name, value, settings...)
	if res.OK() {
		return true, res
	}
	return false, res
}
func (module *serviceModule) Invoking(ctx context, name string, offset, limit int64, value Map, settings ...Map) (int64, []Map, *Res) {
	if value == nil {
		value = Map{}
	}
	value["offset"] = offset
	value["limit"] = limit

	data, res := module.Invoke(ctx, name, value, settings...)
	if res.Fail() {
		return 0, nil, res
	}

	count, countOK := data["count"].(int64)
	items, itemsOK := data["items"].([]Map)
	if countOK && itemsOK {
		return count, items, res
	}

	return 0, []Map{data}, res
}

func (module *serviceModule) Invoker(ctx context, name string, value Map, settings ...Map) (Map, []Map, *Res) {
	data, res := module.Invoke(ctx, name, value, settings...)
	if res.Fail() {
		return nil, nil, res
	}

	item, itemOK := data["item"].(asset.Map)
	items, itemsOK := data["items"].([]asset.Map)

	if itemOK && itemsOK {
		return item, items, res
	}

	return data, []asset.Map{data}, res
}

func (module *serviceModule) Invokee(ctx context, name string, value Map, settings ...Map) (float64, *Res) {
	data, res := module.Invoke(ctx, name, value, settings...)
	if res.Fail() {
		return 0, res
	}

	if vv, ok := data["count"].(float64); ok {
		return vv, res
	} else if vv, ok := data["count"].(int64); ok {
		return float64(vv), res
	}

	return 0, res
}

func (module *serviceModule) Library(name string) *serviceLibrary {
	return &serviceLibrary{module, name}
}
func (module *serviceModule) Logic(ctx context, name string, settings ...Map) *ServiceLogic {
	setting := make(Map)
	if len(settings) > 0 {
		setting = settings[0]
	}
	return &ServiceLogic{ctx, name, setting}
}

//获取参数定义
func (module *serviceModule) Arguments(name string, extends ...Vars) Vars {
	args := Vars{}

	if config, ok := module.methods[name]; ok {
		for k, v := range config.Args {
			args[k] = v
		}
	}
	return VarExtend(args, extends...)
}

//------------ library ----------------

func (lib *serviceLibrary) Name() string {
	return lib.name
}
func (lib *serviceLibrary) Register(name string, config Method, overrides ...bool) {
	realName := fmt.Sprintf("%s.%s", lib.name, name)
	lib.module.Method(realName, config, overrides...)
}

//------------------- Service 方法 --------------------
func (sv *Service) Zone() *time.Location {
	return sv.ctx.Zone()
}
func (sv *Service) Lang() string {
	return sv.ctx.Lang()
}

func (sv *Service) Result() *Res {
	return sv.ctx.Result()
}
func (lgc *Service) Data(bases ...string) DataBase {
	return lgc.ctx.dataBase(bases...)
}
func (service *Service) Invoke(name string, values ...Map) Map {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvv, res := ark.Service.Invoke(service.ctx, name, value)
	service.ctx.Result(res)
	return vvv
}

func (service *Service) Invokes(name string, values ...Map) []Map {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvs, res := ark.Service.Invokes(service.ctx, name, value)
	service.ctx.Result(res)
	return vvs
}
func (service *Service) Invoked(name string, values ...Map) bool {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvv, res := ark.Service.Invoked(service.ctx, name, value)
	service.ctx.Result(res)
	return vvv
}
func (service *Service) Invoking(name string, offset, limit int64, values ...Map) (int64, []Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	count, items, res := ark.Service.Invoking(service.ctx, name, offset, limit, value)
	service.ctx.Result(res)
	return count, items
}

func (service *Service) Invoker(name string, values ...Map) (Map, []Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	item, items, res := ark.Service.Invoker(service.ctx, name, value)
	service.ctx.Result(res)
	return item, items
}

func (service *Service) Invokee(name string, values ...Map) float64 {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	count, res := ark.Service.Invokee(service.ctx, name, value)
	service.ctx.Result(res)
	return count
}

func (lgc *Service) Logic(name string, settings ...Map) *ServiceLogic {
	return ark.Service.Logic(lgc.ctx, name, settings...)
}

//语法糖
func (lgc *Service) Locked(key string, expiry time.Duration, cons ...string) bool {
	return ark.Mutex.Lock(key, expiry, cons...) != nil
}
func (lgc *Service) Lock(key string, expiry time.Duration, cons ...string) error {
	return ark.Mutex.Lock(key, expiry, cons...)
}
func (lgc *Service) Unlock(key string, cons ...string) error {
	return ark.Mutex.Unlock(key, cons...)
}

//------- logic 方法 -------------
func (logic *ServiceLogic) naming(name string) string {
	return logic.Name + "." + name
}

func (logic *ServiceLogic) Invoke(name string, values ...Map) Map {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvv, res := ark.Service.Invoke(logic.ctx, name, value)
	logic.ctx.Result(res)
	return vvv
}

func (logic *ServiceLogic) Invokes(name string, values ...Map) []Map {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvs, res := ark.Service.Invokes(logic.ctx, name, value)
	logic.ctx.Result(res)
	return vvs
}
func (logic *ServiceLogic) Invoked(name string, values ...Map) bool {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	vvv, res := ark.Service.Invoked(logic.ctx, name, value)
	logic.ctx.Result(res)
	return vvv
}
func (logic *ServiceLogic) Invoking(name string, offset, limit int64, values ...Map) (int64, []Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	count, items, res := ark.Service.Invoking(logic.ctx, name, offset, limit, value)
	logic.ctx.Result(res)
	return count, items
}

func (logic *ServiceLogic) Invoker(name string, values ...Map) (Map, []Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	item, items, res := ark.Service.Invoker(logic.ctx, name, value)
	logic.ctx.Result(res)
	return item, items
}

func (logic *ServiceLogic) Invokee(name string, values ...Map) float64 {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	count, res := ark.Service.Invokee(logic.ctx, name, value)
	logic.ctx.Result(res)
	return count
}

//-------------------------------------------------------------------------------------------------------

// func Method(name string, config Map, overrides ...bool) {
// 	ark.Service.Method(name, config, overrides...)
// }

func Library(name string) *serviceLibrary {
	return ark.Service.Library(name)
}

//触发执行，异步
func Arguments(name string, extends ...Vars) Vars {
	return ark.Service.Arguments(name, extends...)
}

//直接执行，同步
func Execute(name string, values ...Map) (Map, *Res) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	return ark.Service.Invoke(nil, name, value)
}

//触发执行，异步
func Trigger(name string, values ...Map) {
	value := Map{}
	if len(values) > 0 {
		value = values[0]
	}
	go ark.Service.Invoke(nil, name, value)
}
