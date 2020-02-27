package ark

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/arkgo/asset/fastid"
	"github.com/arkgo/asset/hashid"
)

type (
	serialConfig struct {
		Text   string `toml:"text"`
		Digit  string `toml:"digit"`
		Salt   string `toml:"salt"`
		Length int    `toml:"length"`

		begin int64
		Start string `toml:"start"`
		Time  uint   `toml:"timebits"`
		Node  uint   `toml:"nodebits"`
		Seq   uint   `toml:"seqbits"`
	}
	serialModule struct {
		config     serialConfig
		fastid     *fastid.FastID
		textCoder  *base64.Encoding
		digitCoder *hashid.HashID
	}
)

func newSerial(config serialConfig) *serialModule {
	if config.Text == "" {
		config.Text = "01234AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz56789+/"
	}
	if config.Digit == "" {
		//这个简化去掉了不容易识别的字符
		config.Digit = "abcdefghijkmnpqrstuvwxyz123456789ACDEFGHJKLMNPQRSTUVWXYZ"
	}
	if config.Length <= 0 {
		config.Length = 7
	}

	if config.Start != "" {
		t, e := time.Parse("2006-01-02", config.Start)
		if e == nil {
			config.begin = t.UnixNano()
		} else {
			config.begin = time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local).UnixNano()
		}
	} else {
		config.begin = time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local).UnixNano()
	}
	if config.Time <= 0 {
		config.Time = 43 //41位=毫秒，约69年可用，42=138年，43=276年，44位=552年
	}
	if config.Node <= 0 {
		config.Node = 7 //8=256
	}
	if config.Seq <= 0 {
		config.Seq = 13 //12=4096，13位=819.2万
	}

	serial := &serialModule{
		config: config,
	}

	serial.fastid = fastid.NewFastIDWithConfig(config.Time, config.Node, config.Seq, config.begin, ark.Node.config.Id)
	serial.textCoder = base64.NewEncoding(config.Text)
	coder, err := hashid.NewWithData(&hashid.HashIDData{
		Alphabet: config.Digit, Salt: config.Salt, MinLength: config.Length,
	})
	if err != nil {
		panic("[序列]无效的配置")
	}
	serial.digitCoder = coder
	return serial
}

func (module *serialModule) Encrypt(text string) string {
	if module.textCoder != nil {
		return module.textCoder.EncodeToString([]byte(text))
	}
	//不返回原文，要不然没法判断成功了没
	//return text
	return ""
}
func (module *serialModule) Decrypt(code string) string {
	if module.textCoder != nil {
		d, e := module.textCoder.DecodeString(code)
		if e == nil {
			return string(d)
		}
	}
	//不能返回原文
	//要不然请求时候的参数要加密，就没意义了
	//return code
	return ""
}
func (module *serialModule) Encrypts(texts []string) string {
	text := strings.Join(texts, "\n")
	if module.textCoder != nil {
		return module.textCoder.EncodeToString([]byte(text))
	}
	return ""
}
func (module *serialModule) Decrypts(code string) []string {
	if module.textCoder != nil {
		text, e := module.textCoder.DecodeString(code)
		if e == nil {
			return strings.Split(string(text), "\n")
		}
	}
	return []string{}
}

func (module *serialModule) Enhash(digit int64, lengths ...int) string {
	return module.Enhashs([]int64{digit}, lengths...)
}
func (module *serialModule) Dehash(code string, lengths ...int) int64 {
	digits := module.Dehashs(code, lengths...)
	if len(digits) > 0 {
		return digits[0]
	} else {
		return int64(-1)
	}
}

//因为要自定义长度，所以动态创建对象
func (module *serialModule) Enhashs(digits []int64, lengths ...int) string {
	coder := module.digitCoder

	if len(lengths) > 0 {
		length := lengths[0]

		hd := hashid.NewData()
		hd.Alphabet = module.config.Digit
		hd.Salt = module.config.Salt
		if length > 0 {
			hd.MinLength = length
		}
		coder, _ = hashid.NewWithData(hd)
	}

	if coder != nil {
		code, err := coder.EncodeInt64(digits)
		if err == nil {
			return code
		}
	}

	return ""
}

//因为要自定义长度，所以动态创建对象
func (module *serialModule) Dehashs(code string, lengths ...int) []int64 {
	coder := module.digitCoder

	if len(lengths) > 0 {
		length := lengths[0]

		hd := hashid.NewData()
		hd.Alphabet = module.config.Digit
		hd.Salt = module.config.Salt
		if length > 0 {
			hd.MinLength = length
		}

		coder, _ = hashid.NewWithData(hd)
	}

	if digits, err := coder.DecodeInt64WithError(code); err == nil {
		return digits
	}

	return []int64{}
}

func (module *serialModule) Serial() int64 {
	return module.fastid.NextID()
}
func (module *serialModule) Unique(prefixs ...string) string {
	id := module.fastid.NextID()
	if len(prefixs) > 0 {
		return fmt.Sprintf("%s%s", prefixs[0], module.Enhash(id))
	} else {
		return module.Enhash(id)
	}
}

func Encrypt(text string) string {
	return ark.Serial.Decrypt(text)
}
func Decrypt(code string) string {
	return ark.Serial.Decrypt(code)
}
func Encrypts(texts []string) string {
	return ark.Serial.Encrypts(texts)
}
func Decrypts(code string) []string {
	ark.Serial.Decrypts(code)
}

func Enhash(digit int64, lengths ...int) string {
	return ark.Serial.Enhash(digit, lengths...)
}
func Dehash(code string, lengths ...int) int64 {
	return ark.Serial.Dehash(code, lengths...)
}

//因为要自定义长度，所以动态创建对象
func Enhashs(digits []int64, lengths ...int) string {
	return ark.Serial.Enhashs(digits, lengths...)
}

//因为要自定义长度，所以动态创建对象
func Dehashs(code string, lengths ...int) []int64 {
	return ark.Serial.Dehashs(code, lengths...)
}

func Serial() int64 {
	return ark.Serial.Serial()
}
func Unique(prefixs ...string) string {
	return ark.Serial.Unique(prefixs...)
}
