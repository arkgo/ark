package ark

import (
	. "github.com/arkgo/base"
)

// Register 是注册新服务
func Register(name string, config Map) {
	ark.Logger.Debug("register service", name)
}
