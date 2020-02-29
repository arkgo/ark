package ark

import (
	"errors"
	"os"
	"path"
	"time"

	"github.com/arkgo/asset/toml"
	. "github.com/arkgo/base"
)

func init() {
	initing()
	builtin()
}

func loading(file string, out Any) error {
	if _, err := os.Stat(file); err == nil {
		_, err := toml.DecodeFile(file, out)
		return err
	}
	return errors.New("nothing to do")
}

func config() *arkConfig {
	config := &arkConfig{}

	cfgfile := "config.toml"
	if len(os.Args) >= 2 {
		cfgfile = os.Args[1]
	}

	var tmp arkConfig
	err := loading(cfgfile, &tmp)
	if err == nil {
		config = &tmp
	}

	//配置的初始设化，默认值
	switch config.Mode {
	case "d", "dev", "develop", "development", "developing":
		Mode = Developing
	case "t", "test", "testing":
		Mode = Testing
	case "p", "pro", "prod", "product", "production":
		Mode = Production
	default:
		Mode = Developing
	}

	//节点默认配置
	if config.Node.Id <= 0 {
		config.Node.Id = 1
	}
	if config.Node.Type == "" {
		config.Node.Type = GATEWAY
	}

	//基础默认配置
	if config.Basic.State == "" {
		config.Basic.State = "asset/state.toml"
	}
	if config.Basic.Mime == "" {
		config.Basic.Mime = "asset/mime.toml"
	}
	if config.Basic.Regular == "" {
		config.Basic.Regular = "asset/regular.toml"
	}
	if config.Basic.Lang == "" {
		config.Basic.Lang = "asset/langs"
	}

	//默认lang驱动
	if config.Lang == nil {
		config.Lang = map[string]langConfig{
			"zh-CN": langConfig{
				Name:    "简体中文",
				Accepts: []string{"zh", "cn", "zh-CN", "zhCN"},
			},
			"zh-TW": langConfig{
				Name:    "繁體中文",
				Accepts: []string{"zh-TW", "zhTW", "tw"},
			},
			"en-US": langConfig{
				Name:    "English",
				Accepts: []string{"en", "en-US"},
			},
		}
	}

	//序列默认配置
	if config.Serial.Text == "" {
		config.Serial.Text = "01234AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz56789+/"
	}
	if config.Serial.Digit == "" {
		//这个简化去掉了不容易识别的字符
		config.Serial.Digit = "abcdefghijkmnpqrstuvwxyz123456789ACDEFGHJKLMNPQRSTUVWXYZ"
	}
	if config.Serial.Length <= 0 {
		config.Serial.Length = 7
	}

	if config.Serial.Start != "" {
		t, e := time.Parse("2006-01-02", config.Serial.Start)
		if e == nil {
			config.Serial.begin = t.UnixNano()
		} else {
			config.Serial.begin = time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local).UnixNano()
		}
	} else {
		config.Serial.begin = time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local).UnixNano()
	}
	if config.Serial.Time <= 0 {
		config.Serial.Time = 43 //41位=毫秒，约69年可用，42=138年，43=276年，44位=552年
	}
	if config.Serial.Node <= 0 {
		config.Serial.Node = 7 //8=256
	}
	if config.Serial.Seq <= 0 {
		config.Serial.Seq = 13 //12=4096，13位=819.2万
	}

	//默认锁配置
	if config.Mutex.Driver == "" {
		config.Mutex.Driver = DEFAULT
	}
	//日志默认配置
	if config.Logger.Driver == "" {
		config.Logger.Driver = DEFAULT
		config.Logger.Console = true
	}

	//总线默认配置
	if config.Bus == nil {
		config.Bus = map[string]BusConfig{
			DEFAULT: BusConfig{
				Driver: DEFAULT, Weight: 1,
			},
		}
	} else {
		for k, v := range config.Bus {
			if v.Driver == "" {
				v.Driver = DEFAULT
			}
			config.Bus[k] = v
		}
	}

	//默认file,strore配置
	if config.File.Sharding <= 0 {
		config.File.Sharding = 2000
	}
	if config.File.Storage == "" {
		config.File.Storage = "store/storage"
	}
	if config.File.Thumbnail == "" {
		config.File.Thumbnail = "store/thumbnail"
	}
	if config.File.Cache == "" {
		config.File.Cache = "store/cache"
	}
	if config.File.Tokens == nil || len(config.File.Tokens) == 0 {
		config.File.Tokens = []string{"expiry", "address"}
	}

	for k, c := range config.Store {
		if c.Cache == "" {
			c.Cache = path.Join(config.File.Cache, k)
		}
		if _, err := os.Stat(c.Cache); err != nil {
			os.MkdirAll(c.Cache, 0777)
		}
		config.Store[k] = c
	}

	//会话默认配置
	if config.Session == nil {
		config.Session = map[string]SessionConfig{
			DEFAULT: SessionConfig{
				Driver: DEFAULT, Weight: 1,
			},
		}
	} else {
		for k, v := range config.Session {
			if v.Driver == "" {
				v.Driver = DEFAULT
			}
			if v.Weight <= 0 {
				v.Weight = 1
			}
			config.Session[k] = v
		}
	}

	//默认缓存配置
	if config.Cache == nil {
		config.Cache = map[string]CacheConfig{
			DEFAULT: CacheConfig{
				Driver: DEFAULT, Weight: 1,
			},
		}
	} else {
		for k, v := range config.Cache {
			if v.Driver == "" {
				v.Driver = DEFAULT
			}
			if v.Weight == 0 {
				//默认参与分布，否则请设置为-1
				v.Weight = 1
			}
			config.Cache[k] = v
		}
	}

	//数据库，没有默认库

	Setting = config.Setting

	return config
}

// func i18n(file string) (Map, error) {
// 	var config Map
// 	_, err := toml.DecodeFile(file, &config)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return config, nil
// }

func initing() {
	ark = &arkCore{}
	ark.Config = config()
	ark.Node = newNode()
	ark.Serial = newSerial()
	ark.Basic = newBasic()
	ark.Logger = newLogger()
	ark.Mutex = newMutex()
	ark.Bus = newBus()
	ark.Store = newStore()
	ark.Session = newSession()
	ark.Cache = newCache()
	ark.Data = newData()
}

func builtin() {
	OK = Result(0, "ok", "成功")
	Fail = Result(-1, "fail", "失败")
	Retry = Result(-2, "retry", "请稍后再试")
}
