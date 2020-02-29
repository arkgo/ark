package ark

import (
	"fmt"
	"strings"
	"sync"
	"time"

	. "github.com/arkgo/base"
)

type (

	// LoggerConfig 是日志配置类
	LoggerConfig struct {
		Driver  string `toml:"driver"`
		Flag    string `toml:"flag"`
		Console bool   `toml:"console"`
		Level   string `toml:"level"`
		Format  string `toml:"format"`
		Setting Map    `toml:"setting"`
	}

	// LoggerDriver 日志驱动
	LoggerDriver interface {
		Connect(config LoggerConfig) (LoggerConnect, error)
	}
	// LoggerConnect 日志连接
	LoggerConnect interface {
		//打开、关闭
		Open() error
		Health() (LoggerHealth, error)
		Close() error

		Debug(string)
		Debugf(string, ...Any)
		Trace(string)
		Tracef(string, ...Any)
		Info(string)
		Infof(string, ...Any)
		Warning(string)
		Warningf(string, ...Any)
		Error(string)
		Errorf(string, ...Any)
	}

	// LoggerHealth 日志健康信息
	LoggerHealth struct {
		Workload int64
	}

	loggerModule struct {
		mutex   sync.Mutex
		drivers map[string]LoggerDriver

		connect LoggerConnect
	}
)

func newLogger() *loggerModule {
	return &loggerModule{
		drivers: map[string]LoggerDriver{},
	}
}

//注册日志驱动
func (module *loggerModule) Driver(name string, driver LoggerDriver, overrides ...bool) {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	if driver == nil {
		panic("[日志]驱动不可为空")
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

func (module *loggerModule) connecting(config LoggerConfig) (LoggerConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(config)
	}
	panic("[日志]不支持的驱动" + config.Driver)
}

//初始化
func (module *loggerModule) initing() {
	connect, err := module.connecting(ark.Config.Logger)
	if err != nil {
		panic("[日志]连接失败：" + err.Error())
	}

	//打开连接
	err = connect.Open()
	if err != nil {
		panic("[日志]打开失败：" + err.Error())
	}

	//保存连接
	module.connect = connect
}

//退出
func (module *loggerModule) exiting() {
	if module.connect != nil {
		module.connect.Close()
	}
}

func (module *loggerModule) formating(args []Any) (string, []Any) {
	format := ""
	if len(args) > 1 {
		if vv, ok := args[0].(string); ok {
			ccc := strings.Count(vv, "%") - strings.Count(vv, "%%")
			if ccc > 0 && ccc == (len(args)-1) {
				format = vv
				args = args[1:]
			}
		}
	}

	return format, args
}
func (module *loggerModule) tostring(args ...Any) string {
	vs := []string{}
	for _, v := range args {
		s := ""
		if m, ok := v.(Map); ok {
			vs := []string{}
			for k, v := range m {
				vs = append(vs, k, fmt.Sprintf("%v", v))
			}
			s = strings.Join(vs, " ")
		} else if ms, ok := v.([]Map); ok {
			vss := []string{}
			for _, m := range ms {
				for k, v := range m {
					vss = append(vss, k, fmt.Sprintf("%v", v))
				}
			}
			s = strings.Join(vs, " ")
		} else {
			s = fmt.Sprintf("%v", v)
		}
		vs = append(vs, s)
	}
	return strings.Join(vs, " ")
}

//output是为了直接输出到控制台，不管是否启用控制台
func (module *loggerModule) output(args ...Any) {
	if ark.Config.Logger.Console == false {
		ts := time.Now().Format("2006-01-02 15:04:05")
		format, args := module.formating(args)
		if format != "" {
			format = ts + " " + format
			s := fmt.Sprintf(format, args...)
			fmt.Println(s)
		} else {
			args2 := []Any{ts}
			args2 = append(args2, args...)
			fmt.Println(args2...)
		}
	}
}

//调试
func (module *loggerModule) Debug(args ...Any) {
	if module.connect != nil {
		format, args := module.formating(args)
		if format != "" {
			module.connect.Debugf(format, args...)
		} else {
			s := module.tostring(args...)
			module.connect.Debug(s)
		}
	} else {
		module.output(args...)
	}
}

//信息
func (module *loggerModule) Trace(args ...Any) {
	if module.connect != nil {
		format, args := module.formating(args)
		if format != "" {
			module.connect.Tracef(format, args...)
		} else {
			s := module.tostring(args...)
			module.connect.Trace(s)
		}
	} else {
		module.output(args...)
	}
}

//信息
func (module *loggerModule) Info(args ...Any) {
	if module.connect != nil {
		format, args := module.formating(args)
		if format != "" {
			module.connect.Infof(format, args...)
		} else {
			s := module.tostring(args...)
			module.connect.Info(s)
		}
	} else {
		module.output(args...)
	}
}

//警告
func (module *loggerModule) Warning(args ...Any) {
	if module.connect != nil {
		format, args := module.formating(args)
		if format != "" {
			module.connect.Warningf(format, args...)
		} else {
			s := module.tostring(args...)
			module.connect.Warning(s)
		}
	} else {
		module.output(args...)
	}
}

//错误
func (module *loggerModule) Error(args ...Any) {
	if module.connect != nil {
		format, args := module.formating(args)
		if format != "" {
			module.connect.Errorf(format, args...)
		} else {
			s := module.tostring(args...)
			module.connect.Error(s)
		}
	} else {
		module.output(args...)
	}
}

//语法糖

func Debug(args ...Any) {
	ark.Logger.Debug(args...)
}
func Trace(args ...Any) {
	ark.Logger.Trace(args...)
}
func Info(args ...Any) {
	ark.Logger.Info(args...)
}
func Warning(args ...Any) {
	ark.Logger.Warning(args...)
}
func Error(args ...Any) {
	ark.Logger.Error(args...)
}
