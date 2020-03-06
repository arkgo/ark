package ark

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/arkgo/asset/fastid"
	"github.com/arkgo/asset/hashid"

	jsoniter "github.com/json-iterator/go"
)

type (
	serialConfig struct {
		Text   string `toml:"text"`
		Digit  string `toml:"digit"`
		Salt   string `toml:"salt"`
		Length int    `toml:"length"`

		begin    int64
		Start    string `toml:"start"`
		TimeBits uint   `toml:"timeBits"`
		NodeBits uint   `toml:"nodeBits"`
		SeqBits  uint   `toml:"seqBits"`
	}
	serialModule struct {
		config     serialConfig
		fastid     *fastid.FastID
		textCoder  *base64.Encoding
		digitCoder *hashid.HashID
		jsonCodec  jsoniter.API
	}
)

func newSerial() *serialModule {
	serial := &serialModule{}

	serial.fastid = fastid.NewFastIDWithConfig(ark.Config.Serial.TimeBits, ark.Config.Serial.NodeBits, ark.Config.Serial.SeqBits, ark.Config.Serial.begin, ark.Config.Node.Id)
	serial.textCoder = base64.NewEncoding(ark.Config.Serial.Text)
	coder, err := hashid.NewWithData(&hashid.HashIDData{
		Alphabet: ark.Config.Serial.Digit, Salt: ark.Config.Serial.Salt, MinLength: ark.Config.Serial.Length,
	})
	if err != nil {
		panic("[序列]无效的配置")
	}
	serial.digitCoder = coder

	serial.jsonCodec = jsoniter.ConfigCompatibleWithStandardLibrary

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
		hd.Alphabet = ark.Config.Serial.Digit
		hd.Salt = ark.Config.Serial.Salt
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
		hd.Alphabet = ark.Config.Serial.Digit
		hd.Salt = ark.Config.Serial.Salt
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

func (module *serialModule) Marshal(v interface{}) ([]byte, error) {
	return module.jsonCodec.Marshal(v)
}
func (module *serialModule) Unmarshal(data []byte, v interface{}) error {
	return module.jsonCodec.Unmarshal(data, v)
}

func Encrypt(text string) string {
	return ark.Serial.Encrypt(text)
}
func Decrypt(code string) string {
	return ark.Serial.Decrypt(code)
}
func Encrypts(texts []string) string {
	return ark.Serial.Encrypts(texts)
}
func Decrypts(code string) []string {
	return ark.Serial.Decrypts(code)
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



func  Marshal(v interface{}) ([]byte, error) {
	return ark.Serial.jsonCodec.Marshal(v)
}
func Unmarshal(data []byte, v interface{}) error {
	return ark.Serial.jsonCodec.Unmarshal(data, v)
}



func TextAlphabet() string {
	return ark.Serial.config.Text
}
func DigitAlphabet() string {
	return ark.Serial.config.Digit
}
