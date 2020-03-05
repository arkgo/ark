package ark

import (
	. "github.com/arkgo/base"
)

type (
	ViewConfig struct {
		Driver  string `toml:"driver"`
		Path    string `toml:"path"`
		Left    string `toml:"left"`
		Right   string `toml:"right"`
		Setting Map    `toml:"setting"`
	}
	//视图驱动
	ViewDriver interface {
		Connect(config ViewConfig) (ViewConnect, error)
	}
	//视图连接
	ViewConnect interface {
		//打开、关闭
		Open() error
		Health() (ViewHealth, error)
		Close() error

		// Parse(*Http, ViewBody) (string, error)
	}

	ViewHealth struct {
		Workload int64
	}
)
