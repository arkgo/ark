package ark

import (
	"errors"
	"os"
	"path"
	"strings"
	"time"

	. "github.com/arkgo/asset"
	"github.com/arkgo/asset/toml"
)

func init() {
	build()
}

func loading(file string, out Any) error {
	if _, err := os.Stat(file); err == nil {
		_, err := toml.DecodeFile(file, out)
		return err
	}
	return errors.New("nothing to do")
}

func config() *arkConfig {
	config := &arkConfig{
		Name: "ark", Mode: "dev",
		hosts: make(map[string]string),
	}

	cfgfile := "config.toml"
	if len(os.Args) >= 2 {
		cfgfile = os.Args[1]
	}

	var tmp arkConfig
	err := loading(cfgfile, &tmp)
	if err != nil {
		panic("加载配置文件失败")
	}
	config = &tmp

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
			"en": langConfig{
				Name:    "English",
				Accepts: []string{"en", "en-US"},
			},
		}
	}

	//序列默认配置
	if config.Codec.Text == "" {
		config.Codec.Text = "01234AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz56789+/"
	}
	if config.Codec.Digit == "" {
		//这个简化去掉了不容易识别的字符
		config.Codec.Digit = "abcdefghijkmnpqrstuvwxyz123456789ACDEFGHJKLMNPQRSTUVWXYZ"
	}
	if config.Codec.Length <= 0 {
		config.Codec.Length = 7
	}

	if config.Codec.Start != "" {
		t, e := time.Parse("2006-01-02", config.Codec.Start)
		if e == nil {
			config.Codec.begin = t.UnixNano()
		} else {
			config.Codec.begin = time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local).UnixNano()
		}
	} else {
		config.Codec.begin = time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local).UnixNano()
	}
	if config.Codec.TimeBits <= 0 {
		config.Codec.TimeBits = 43 //41位=毫秒，约69年可用，42=138年，43=276年，44位=552年
	}
	if config.Codec.NodeBits <= 0 {
		config.Codec.NodeBits = 7 //8=256
	}
	if config.Codec.SeqBits <= 0 {
		config.Codec.SeqBits = 13 //12=4096，13位=819.2万
	}

	//默认锁配置
	if config.Mutex == nil {
		config.Mutex = map[string]MutexConfig{
			DEFAULT: {
				Driver: DEFAULT, Weight: 1, Expiry: "2s",
			},
		}
	} else {
		for k, v := range config.Mutex {
			if v.Driver == "" {
				v.Driver = DEFAULT
			}
			if v.Weight == 0 {
				//默认参与分布，否则请设置为-1
				v.Weight = 1
			}
			config.Mutex[k] = v
		}
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
			if v.Weight == 0 {
				v.Weight = 1
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
	for k, v := range config.Data {
		if v.Weight == 0 {
			//默认参与分布，否则请设置为-1
			v.Weight = 1
		}
		config.Data[k] = v
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
			if v.Weight == 0 {
				//默认参与分布，否则请设置为-1
				v.Weight = 1
			}
			config.Session[k] = v
		}
	}

	//默认HTTP驱动
	if config.Http.Driver == "" {
		config.Http.Driver = DEFAULT
	}
	if config.Http.Port <= 0 || config.Http.Port > 65535 {
		config.Http.Port = 80
	}
	if config.Http.Charset == "" {
		config.Http.Charset = "utf-8"
	}
	if config.Http.Expiry == "" {
		config.Http.Expiry = "30d"
	}
	if config.Http.MaxAge == "" {
		config.Http.MaxAge = "365d"
	}
	if config.Http.Upload == "" {
		config.Http.Upload = os.TempDir()
	}
	if config.Http.Static == "" {
		config.Http.Static = "asset/statics"
	}
	if config.Http.Shared == "" {
		config.Http.Shared = "shared"
	}

	//http默认驱动
	for k, v := range config.Site {
		if v.Charset == "" {
			v.Charset = config.Http.Charset
		}
		if v.Domain == "" {
			v.Domain = config.Http.Domain
		}
		if v.Cookie == "" {
			v.Cookie = config.Name
		}
		if v.Expiry == "" {
			v.Expiry = config.Http.Expiry
		}
		if v.MaxAge == "" {
			v.MaxAge = config.Http.MaxAge
		}
		if v.Hosts == nil {
			v.Hosts = []string{}
		}
		if v.Host != "" {
			if strings.Contains(v.Host, ".") {
				v.Hosts = append(v.Hosts, v.Host)
			} else {
				v.Hosts = append(v.Hosts, v.Host+"."+v.Domain)
			}
		} else {
			if len(v.Hosts) == 0 {
				v.Hosts = append(v.Hosts, k+"."+v.Domain)
			}
		}

		//还没有设置域名，自动来一波
		if len(v.Hosts) == 0 && v.Domain != "" {
			v.Hosts = append(v.Hosts, k+"."+v.Domain)
		}
		//待处理，这个权重是老代码复制，暂时不知道干什么用，
		if v.Weights == nil || len(v.Weights) == 0 {
			v.Weights = []int{}
			for range v.Hosts {
				v.Weights = append(v.Weights, 1)
			}
		}

		if v.Format == "" {
			v.Format = `{device}/{system}/{version}/{client}/{number}/{time}/{path}`
		}

		//记录http的所有域名
		config.hosts = make(map[string]string)
		for _, host := range v.Hosts {
			config.hosts[host] = k
		}

		config.Site[k] = v
	}

	//隐藏的空站点，不接域名
	if config.Site == nil {
		config.Site = make(map[string]SiteConfig)
	}
	config.Site[""] = SiteConfig{Name: "空站点"}

	//默认view驱动
	if config.View.Driver == "" {
		config.View.Driver = DEFAULT
	}
	if config.View.Root == "" {
		config.View.Root = "asset/views"
	}
	if config.View.Shared == "" {
		config.View.Shared = "shared"
	}

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

func build() {
	ark = &arkCore{}

	ark.Config = config()
	ark.Node = newNode()
	ark.Codec = newCodec()
	ark.Basic = newBasic()

	ark.Logger = newLogger()
	ark.Mutex = newMutex()

	ark.Gateway = newGateway()
	ark.Service = newService()

	ark.Bus = newBus()
	ark.Store = newStore()
	ark.Cache = newCache()
	ark.Data = newData()
	ark.Session = newSession()
	ark.Http = newHttp()
	ark.View = newView()

	Sites = Site("*")
	Root = Site("")

	OK = Result(0, "ok", "成功")
	Fail = Result(-1, "fail", "失败")
	Found = Result(-2, "found", "不存在")
	Retry = Result(-3, "retry", "请稍后再试")
	Invalid = Result(-4, "invalid", "无效数据或请求")
}
