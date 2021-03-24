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
	codecConfig struct {
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
	codecModule struct {
		// config     codecConfig
		fastid     *fastid.FastID
		textCoder  *base64.Encoding
		digitCoder *hashid.HashID
		jsonCodec  jsoniter.API
	}
)

func newCodec() *codecModule {
	codec := &codecModule{}

	codec.fastid = fastid.NewFastIDWithConfig(ark.Config.Codec.TimeBits, ark.Config.Codec.NodeBits, ark.Config.Codec.SeqBits, ark.Config.Codec.begin, ark.Config.Node.Id)
	codec.textCoder = base64.NewEncoding(ark.Config.Codec.Text)
	coder, err := hashid.NewWithData(&hashid.HashIDData{
		Alphabet: ark.Config.Codec.Digit, Salt: ark.Config.Codec.Salt, MinLength: ark.Config.Codec.Length,
	})
	if err != nil {
		panic("[序列]无效的配置")
	}
	codec.digitCoder = coder

	//codec.jsonCodec = jsoniter.ConfigCompatibleWithStandardLibrary
	codec.jsonCodec = jsoniter.ConfigFastest

	return codec
}

func (module *codecModule) Encrypt(text string) string {
	if module.textCoder != nil {
		return module.textCoder.EncodeToString([]byte(text))
	}
	//不返回原文，要不然没法判断成功了没
	//return text
	return ""
}
func (module *codecModule) Decrypt(code string) string {
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
func (module *codecModule) Encrypts(texts []string) string {
	text := strings.Join(texts, "\n")
	if module.textCoder != nil {
		return module.textCoder.EncodeToString([]byte(text))
	}
	return ""
}
func (module *codecModule) Decrypts(code string) []string {
	if module.textCoder != nil {
		text, e := module.textCoder.DecodeString(code)
		if e == nil {
			return strings.Split(string(text), "\n")
		}
	}
	return []string{}
}

func (module *codecModule) Enhash(digit int64, lengths ...int) string {
	return module.Enhashs([]int64{digit}, lengths...)
}
func (module *codecModule) Dehash(code string, lengths ...int) int64 {
	digits := module.Dehashs(code, lengths...)
	if len(digits) > 0 {
		return digits[0]
	} else {
		return int64(-1)
	}
}

//因为要自定义长度，所以动态创建对象
func (module *codecModule) Enhashs(digits []int64, lengths ...int) string {
	coder := module.digitCoder

	if len(lengths) > 0 {
		length := lengths[0]

		hd := hashid.NewData()
		hd.Alphabet = ark.Config.Codec.Digit
		hd.Salt = ark.Config.Codec.Salt
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
func (module *codecModule) Dehashs(code string, lengths ...int) []int64 {
	coder := module.digitCoder

	if len(lengths) > 0 {
		length := lengths[0]

		hd := hashid.NewData()
		hd.Alphabet = ark.Config.Codec.Digit
		hd.Salt = ark.Config.Codec.Salt
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

func (module *codecModule) Serial() int64 {
	return module.fastid.NextID()
}
func (module *codecModule) Unique(prefixs ...string) string {
	id := module.fastid.NextID()
	if len(prefixs) > 0 {
		return fmt.Sprintf("%s%s", prefixs[0], module.Enhash(id))
	} else {
		return module.Enhash(id)
	}
}

func (module *codecModule) Marshal(v interface{}) ([]byte, error) {
	return module.jsonCodec.Marshal(v)
}
func (module *codecModule) Unmarshal(data []byte, v interface{}) error {
	return module.jsonCodec.Unmarshal(data, v)
}

func Encrypt(text string) string {
	return ark.Codec.Encrypt(text)
}
func Decrypt(code string) string {
	return ark.Codec.Decrypt(code)
}
func Encrypts(texts []string) string {
	return ark.Codec.Encrypts(texts)
}
func Decrypts(code string) []string {
	return ark.Codec.Decrypts(code)
}

func Enhash(digit int64, lengths ...int) string {
	return ark.Codec.Enhash(digit, lengths...)
}
func Dehash(code string, lengths ...int) int64 {
	return ark.Codec.Dehash(code, lengths...)
}

//因为要自定义长度，所以动态创建对象
func Enhashs(digits []int64, lengths ...int) string {
	return ark.Codec.Enhashs(digits, lengths...)
}

//因为要自定义长度，所以动态创建对象
func Dehashs(code string, lengths ...int) []int64 {
	return ark.Codec.Dehashs(code, lengths...)
}

func Serial() int64 {
	return ark.Codec.Serial()
}
func Unique(prefixs ...string) string {
	return ark.Codec.Unique(prefixs...)
}

func Marshal(v interface{}) ([]byte, error) {
	return ark.Codec.jsonCodec.Marshal(v)
}
func Unmarshal(data []byte, v interface{}) error {
	return ark.Codec.jsonCodec.Unmarshal(data, v)
}

func TextAlphabet() string {
	return ark.Config.Codec.Text
}
func DigitAlphabet() string {
	return ark.Config.Codec.Digit
}
