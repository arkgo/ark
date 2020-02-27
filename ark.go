package ark

import (
	. "github.com/arkgo/base"
)

type (
	arkModule struct {
		Node   *nodeModule
		Serial *serialModule
		Logger *loggerModule
		Bus    *busModule
	}
)

var (
	ark *arkModule
)

func arking() {
	ark = &arkModule{}
	ark.Node = newNode(nodeConfig{})
	ark.Serial = newSerial(serialConfig{})
	ark.Logger = newLogger(LoggerConfig{})
	ark.Bus = newBus(nil)
}

// Run 是启动方法
func Run() {
	ark.Logger.Debug("ark running")
}

func Driver(name string, driver Any) {
	switch drv := driver.(type) {
	case LoggerDriver:
		ark.Logger.Driver(name, drv)
	case BusDriver:
		ark.Bus.Driver(name, drv)
	}
}
