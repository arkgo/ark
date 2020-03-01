package ark

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	. "github.com/arkgo/base"
)

type (
	DataConfig struct {
		Driver  string `toml:"driver"`
		Cache   string `toml:"cache"`
		Url     string `toml:"url"`
		Serial  string `toml:"serial"`
		Setting Map    `toml:"setting"`
	}

	DataDriver interface {
		Connect(name string, config DataConfig) (DataConnect, error)
	}
	DataConnect interface {
		Open() error
		Health() (DataHealth, error)
		Close() error

		Base() DataBase
	}

	DataBase interface {
		Close() error
		Erred() error

		Table(name string) DataTable
		View(name string) DataView
		Model(name string) DataModel
		Serial(key string, start, step int64) int64

		//开启手动提交事务模式
		Begin() (*sql.Tx, error)
		Submit() error
		Cancel() error
	}

	DataTable interface {
		Create(Map) Map
		Change(Map, Map) Map
		Remove(...Any) Map
		Update(sets Map, args ...Any) int64
		Delete(args ...Any) int64

		Entity(Any) Map
		Count(args ...Any) float64
		First(args ...Any) Map
		Query(args ...Any) []Map
		Limit(offset, limit Any, args ...Any) (int64, []Map)
		Group(field string, args ...Any) []Map
	}

	//数据视图接口
	DataView interface {
		Count(args ...Any) float64
		First(args ...Any) Map
		Query(args ...Any) []Map
		Limit(offset, limit Any, args ...Any) (int64, []Map)
		Group(field string, args ...Any) []Map
	}

	//数据模型接口
	DataModel interface {
		First(args ...Any) Map
		Query(args ...Any) []Map
	}

	DataHealth struct {
		Workload int64
	}
	DataTrigger struct {
		Name  string
		Value Map
	}
)

type (
	dataModule struct {
		mutex   sync.Mutex
		drivers map[string]DataDriver
		tables  map[string]Map
		views   map[string]Map
		models  map[string]Map

		//连接
		connects map[string]DataConnect
	}

	dataGroup struct {
		data *dataModule
		base string
	}
)

func newData() *dataModule {
	return &dataModule{
		drivers:  make(map[string]DataDriver, 0),
		tables:   make(map[string]Map, 0),
		views:    make(map[string]Map, 0),
		models:   make(map[string]Map, 0),
		connects: make(map[string]DataConnect, 0),
	}
}

func (module *dataModule) connecting(name string, config DataConfig) (DataConnect, error) {
	if driver, ok := module.drivers[name]; ok {
		return driver.Connect(name, config)
	}
	panic("[数据]不支持的驱动：" + config.Driver)
}

//初始化
func (module *dataModule) initing() {

	for name, config := range ark.Config.Data {

		//连接
		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[数据]连接失败：" + err.Error())
		}

		//打开连接
		err = connect.Open()
		if err != nil {
			panic("[数据]打开失败：" + err.Error())
		}

		module.connects[name] = connect
	}
}

//退出
func (module *dataModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

func (module *dataModule) Driver(name string, driver DataDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[数据]驱动不可为空")
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

//注册表
func (module *dataModule) Table(name string, configs ...Map) Map {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if len(configs) > 0 {
		module.tables[name] = configs[0]
	} else {
		if vv, ok := module.tables[name]; ok {
			return vv
		}
	}

	return nil
}

//注册视图
func (module *dataModule) View(name string, configs ...Map) Map {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if len(configs) > 0 {
		module.views[name] = configs[0]
	} else {
		if vv, ok := module.views[name]; ok {
			return vv
		}
	}

	return nil
}

//注册模型
func (module *dataModule) Model(name string, configs ...Map) Map {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if len(configs) > 0 {
		module.models[name] = configs[0]
	} else {
		if vv, ok := module.models[name]; ok {
			return vv
		}
	}

	return nil
}

func (module *dataModule) Field(name string, field string, exts ...Map) Map {
	fields := module.Fields(name, []string{field}, exts...)
	config := Map{}
	if vv, ok := fields[field].(Map); ok {
		//复制一份，要不然会改写配置
		for k, v := range vv {
			config[k] = v
		}
	}

	if len(exts) > 0 {
		for k, v := range exts[0] {
			config[k] = v
		}
	}
	return config
}
func (module *dataModule) Fields(name string, keys []string, exts ...Map) Map {
	if _, ok := module.tables[name]; ok {
		return module.TableFields(name, keys, exts...)
	} else if _, ok := module.views[name]; ok {
		return module.ViewFields(name, keys, exts...)
	} else if _, ok := module.models[name]; ok {
		return module.ModelFields(name, keys, exts...)
	} else {
		return Map{}
	}
}
func (module *dataModule) TableFields(name string, keys []string, exts ...Map) Map {
	m := Map{}
	if config, ok := module.tables[name]; ok {
		if fields, ok := config["fields"].(Map); ok {
			// if keys==nil || len(keys) == 0 {
			//空数组一个也不返
			if keys == nil {
				for k, v := range fields {
					m[k] = v
				}
			} else {
				for _, k := range keys {
					if v, ok := fields[k]; ok {
						m[k] = v
					}

				}
			}
		}
	}

	return m
}
func (module *dataModule) ViewFields(name string, keys []string, exts ...Map) Map {
	m := Map{}
	if config, ok := module.views[name]; ok {
		if fields, ok := config["fields"].(Map); ok {
			// if keys==nil || len(keys) == 0 {
			//空数组一个也不返
			if keys == nil {
				for k, v := range fields {
					m[k] = v
				}
			} else {
				for _, k := range keys {
					if v, ok := fields[k]; ok {
						m[k] = v
					}

				}
			}
		}
	}

	return m
}
func (module *dataModule) ModelFields(name string, keys []string, exts ...Map) Map {
	m := Map{}
	if config, ok := module.models[name]; ok {
		if fields, ok := config["fields"].(Map); ok {
			// if keys==nil || len(keys) == 0 {
			//空数组一个也不返
			if keys == nil {
				for k, v := range fields {
					m[k] = v
				}
			} else {
				for _, k := range keys {
					if v, ok := fields[k]; ok {
						m[k] = v
					}

				}
			}
		}
	}

	return m
}
func (module *dataModule) Enum(name, field, key string) string {
	enums := ark.Data.Enums(name, field)
	if vv, ok := enums[key]; ok {
		return fmt.Sprintf("%v", vv)
	}
	return key
}
func (module *dataModule) Enums(name, field string) Map {
	if _, ok := module.tables[name]; ok {
		return module.TableEnums(name, field)
	} else if _, ok := module.views[name]; ok {
		return module.ViewEnums(name, field)
	} else if _, ok := module.models[name]; ok {
		return module.ModelEnums(name, field)
	} else {
		return Map{}
	}
}
func (module *dataModule) TableEnums(name, field string) Map {
	m := Map{}

	if config, ok := module.tables[name]; ok {
		if fields, ok := config["fields"].(Map); ok {
			if field, ok := fields[field].(Map); ok {
				if enums, ok := field["enum"].(Map); ok {
					for k, v := range enums {
						m[k] = v
					}
				}
			}
		}
	}

	return m
}
func (module *dataModule) ViewEnums(name, field string) Map {
	m := Map{}

	if config, ok := module.views[name]; ok {
		if fields, ok := config["fields"].(Map); ok {
			if field, ok := fields[field].(Map); ok {
				if enums, ok := field["enum"].(Map); ok {
					for k, v := range enums {
						m[k] = v
					}
				}
			}
		}
	}

	return m
}
func (module *dataModule) ModelEnums(name, field string) Map {
	m := Map{}

	if config, ok := module.models[name]; ok {
		if fields, ok := config["fields"].(Map); ok {
			if field, ok := fields[field].(Map); ok {
				if enums, ok := field["enum"].(Map); ok {
					for k, v := range enums {
						m[k] = v
					}
				}
			}
		}
	}

	return m
}

//返回数据Base对象
func (module *dataModule) Base(names ...string) DataBase {
	if len(names) > 0 {
		name := names[0]
		if connect, ok := module.connects[name]; ok {
			return connect.Base()
		}
	} else {
		for _, connect := range module.connects {
			return connect.Base()
		}
	}

	panic("[数据]无效数据库连接")
}

//----------------------------------------------------------------------

//查询语法解析器
// 字段包裹成  $field$ 请自行处理
// 如mysql为反引号`field`，postgres, oracle为引号"field"，
// 所有参数使问号(?)
// postgres驱动需要自行处理转成 $1,$2这样的
// oracle驱动需要自行处理转成 :1 :2这样的
//mongodb不适用，需驱动自己实现
func (module *dataModule) Parse(args ...Any) (string, []Any, string, error) {

	if len(args) > 0 {

		//如果直接写sql
		if v, ok := args[0].(string); ok {
			sql := v
			params := []interface{}{}
			orderBy := ""

			for i, arg := range args {
				if i > 0 {
					params = append(params, arg)
				}
			}

			//这里要处理一下，把order提取出来
			//先拿到 order by 的位置
			i := strings.Index(strings.ToLower(sql), "order by")
			if i >= 0 {
				orderBy = sql[i:]
				sql = sql[:i]
			}

			return sql, params, orderBy, nil

		} else {

			maps := []Map{}
			for _, v := range args {
				if m, ok := v.(Map); ok {
					maps = append(maps, m)
				}
				//如果直接是[]Map，应该算OR处理啊，暂不处理这个
			}

			querys, values, orders := module.parsing(maps...)

			orderStr := ""
			if len(orders) > 0 {
				orderStr = fmt.Sprintf("ORDER BY %s", strings.Join(orders, ","))
			}

			//sql := fmt.Sprintf("%s %s", strings.Join(querys, " OR "), orderStr)

			if len(querys) == 0 {
				querys = append(querys, "1=1")
			}

			return strings.Join(querys, " OR "), values, orderStr, nil
		}
	} else {
		return "1=1", []Any{}, "", nil
	}
}

//注意，这个是实际的解析，支持递归
func (module *dataModule) parsing(args ...Map) ([]string, []interface{}, []string) {

	fp := DELIMS

	querys := []string{}
	values := make([]interface{}, 0)
	orders := []string{}

	//否则是多个map,单个为 与, 多个为 或
	for _, m := range args {
		ands := []string{}

		for k, v := range m {

			// 字段名处理
			// 包含.应该是处理成json
			// 包含:就处理成数组
			if dots := strings.Split(k, ":"); len(dots) >= 2 {
				k = fmt.Sprintf(`%v%v%v[%v]`, fp, dots[0], fp, dots[1])
			} else {
				k = fmt.Sprintf(`%v%v%v`, fp, k, fp)
			}

			//如果值是ASC,DESC，表示是排序
			//if ov,ok := v.(string); ok && (ov==ASC || ov==DESC) {
			if v == ASC {
				//正序
				orders = append(orders, fmt.Sprintf(`%s ASC`, k))
			} else if v == DESC {
				//倒序
				orders = append(orders, fmt.Sprintf(`%s DESC`, k))

			} else if v == RAND {
				//随机排序
				orders = append(orders, fmt.Sprintf(`%s ASC`, RANDBY))

			} else if v == nil {
				ands = append(ands, fmt.Sprintf(`%s IS NULL`, k))
			} else if v == NIL {
				ands = append(ands, fmt.Sprintf(`%s IS NULL`, k))
			} else if v == NOL {
				//不为空值
				ands = append(ands, fmt.Sprintf(`%s IS NOT NULL`, k))
				/*
				   }  else if _,ok := v.(Nil); ok {
				       //为空值
				       ands = append(ands, fmt.Sprintf(`%s IS NULL`, k))
				   } else if _,ok := v.(NotNil); ok {
				       //不为空值
				       ands = append(ands, fmt.Sprintf(`%s IS NOT NULL`, k))
				   } else if fts,ok := v.(FTS); ok {
				       //处理模糊搜索，此条后续版本会移除
				       safeFts := strings.Replace(string(fts), "'", "''", -1)
				       ands = append(ands, fmt.Sprintf(`%s LIKE '%%%s%%'`, k, safeFts))
				*/
			} else if ms, ok := v.([]Map); ok {
				//是[]Map，相当于or

				qs, vs, os := module.parsing(ms...)
				if len(qs) > 0 {
					ands = append(ands, fmt.Sprintf("(%s)", strings.Join(qs, " OR ")))
					for _, vsVal := range vs {
						values = append(values, vsVal)
					}
				}
				for _, osVal := range os {
					orders = append(orders, osVal)
				}

			} else if opMap, opOK := v.(Map); opOK {
				//v要处理一下如果是map要特别处理
				//key做为操作符，比如 > < >= 等
				//而且多个条件是and，比如 views > 1 AND views < 100
				//自定义操作符的时候，可以用  is not null 吗？
				//hai yao chu li INC in change update

				opAnds := []string{}
				for opKey, opVal := range opMap {
					//这里要支持LIKE
					if opKey == LIKE {
						safeFts := strings.Replace(fmt.Sprintf("%v", opVal), "'", "''", -1)
						opAnds = append(opAnds, fmt.Sprintf(`%s LIKE '%%%s%%'`, k, safeFts))
					} else if opKey == FULL {
						safeFts := strings.Replace(fmt.Sprintf("%v", opVal), "'", "''", -1)
						opAnds = append(opAnds, fmt.Sprintf(`%s LIKE '%%%s%%'`, k, safeFts))
					} else if opKey == LEFT {
						safeFts := strings.Replace(fmt.Sprintf("%v", opVal), "'", "''", -1)
						opAnds = append(opAnds, fmt.Sprintf(`%s LIKE '%s%%'`, k, safeFts))
					} else if opKey == RIGHT {
						safeFts := strings.Replace(fmt.Sprintf("%v", opVal), "'", "''", -1)
						opAnds = append(opAnds, fmt.Sprintf(`%s LIKE '%%%s'`, k, safeFts))
					} else if opKey == ANY {
						opAnds = append(opAnds, fmt.Sprintf(`? = ANY(%s)`, k))
						values = append(values, opVal)
					} else if opKey == IN {
						//IN (?,?,?)

						realArgs := []string{}
						realVals := []Any{}
						switch vs := opVal.(type) {
						case []int:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						case []int64:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						case []string:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						case []Any:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						default:
							realArgs = append(realArgs, "?")
							realVals = append(realVals, vs)
						}

						opAnds = append(opAnds, fmt.Sprintf(`%s IN(%s)`, k, strings.Join(realArgs, ",")))
						for _, v := range realVals {
							values = append(values, v)
						}

					} else if opKey == NIN {
						//NOT IN (?,?,?)

						realArgs := []string{}
						realVals := []Any{}
						switch vs := opVal.(type) {
						case []int:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						case []int64:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						case []string:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						case []Any:
							if len(vs) > 0 {
								for _, v := range vs {
									realArgs = append(realArgs, "?")
									realVals = append(realVals, v)
								}
							} else {
								realArgs = append(realArgs, "?")
								realVals = append(realVals, 0)
							}
						default:
							realArgs = append(realArgs, "?")
							realVals = append(realVals, vs)
						}

						opAnds = append(opAnds, fmt.Sprintf(`%s NOT IN(%s)`, k, strings.Join(realArgs, ",")))
						for _, v := range realVals {
							values = append(values, v)
						}

					} else {
						opAnds = append(opAnds, fmt.Sprintf(`%s %s ?`, k, opKey))
						values = append(values, opVal)
					}
				}

				ands = append(ands, fmt.Sprintf("(%s)", strings.Join(opAnds, " AND ")))

			} else {
				ands = append(ands, fmt.Sprintf(`%s = ?`, k))
				values = append(values, v)
			}
		}

		if len(ands) > 0 {
			querys = append(querys, fmt.Sprintf("(%s)", strings.Join(ands, " AND ")))
		}
	}

	return querys, values, orders
}

func (module *dataModule) newGroup(base string) *dataGroup {
	return &dataGroup{module, base}
}
func (group *dataGroup) Table(name string, config Map) {
	realName := fmt.Sprintf("%s.%s", group.base, name)
	group.data.Table(realName, config)
}
func (group *dataGroup) View(name string, config Map) {
	realName := fmt.Sprintf("%s.%s", group.base, name)
	group.data.View(realName, config)
}
func (group *dataGroup) Model(name string, config Map) {
	realName := fmt.Sprintf("%s.%s", group.base, name)
	group.data.Model(realName, config)
}

func Data(names ...string) DataBase {
	return ark.Data.Base(names...)
}
func Base(name string) *dataGroup {
	return ark.Data.newGroup(name)
}
func Table(name string, configs ...Map) Map {
	return ark.Data.Table(name, configs...)
}
func View(name string, configs ...Map) Map {
	return ark.Data.View(name, configs...)
}
func Model(name string, configs ...Map) Map {
	return ark.Data.Model(name, configs...)
}
func Field(name string, field string, exts ...Map) Map {
	return ark.Data.Field(name, field, exts...)
}
func Fields(name string, keys []string, exts ...Map) Map {
	return ark.Data.Fields(name, keys, exts...)
}
func Enums(name string, field string) Map {
	return ark.Data.Enums(name, field)
}
func Enum(name, field, key string) string {
	return ark.Data.Enum(name, field, key)
}
func Parse(args ...Any) (string, []Any, string, error) {
	return ark.Data.Parse(args...)
}