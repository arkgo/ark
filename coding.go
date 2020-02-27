package ark

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arkgo/asset/util"
	. "github.com/arkgo/base"
)

type (
	Coding struct {
		Base string
		Type string
		Hash string
		Size int64
	}
	File interface {
		Code() string
		Hash() string
		Name() string
		Size() int64
	}
	Files []File

	arkFile struct {
		code string
		hash string
		name string
		size int64
	}
)

//长十几位无所谓了
func (coding *Coding) Code() string {
	//if len(coding.Hash) < 32 {
	return Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", coding.Base, coding.Type, coding.Hash, coding.Size))
	//}
	//return coding.Hash + "$" + Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", coding.Base, coding.Type, "", coding.Size))
}

func (coding *Coding) Mimetype() string {
	if coding != nil {
		return mBase.Typemime(coding.Type)
	}
	return ""
}

func (data *Coding) Fullname() string {
	if data != nil {
		if data.Type != "" {
			return fmt.Sprintf("%s.%s", data.Hash, data.Type)
		}
		return data.Hash
	}
	return ""
}

//func (coding *Coding) Node() int {
//	if vv,err := strconv.ParseInt(coding.Base, 10, 32); err == nil {
//		return int(vv)
//	}
//	return 0
//}

func (coding *Coding) isFile() bool {
	return coding.Base == ""
}
func (coding *Coding) isStore() bool {
	return coding.Base != ""
}

func (coding *Coding) IsImage() bool {
	return coding.Type == "jpg" ||
		coding.Type == "png" ||
		coding.Type == "gif" ||
		coding.Type == "bmp"
}
func (coding *Coding) IsVideo() bool {
	return coding.Type == "mp4" ||
		coding.Type == "mkv" ||
		coding.Type == "wmv" ||
		coding.Type == "ts" ||
		coding.Type == "mpeg" ||
		coding.Type == "ts"
}

func NewFile(code, hash, name string, size int64) File {
	return &arkFile{code, hash, name, size}
}
func (file *arkFile) Code() string {
	return file.code
}
func (file *arkFile) Hash() string {
	return file.hash
}
func (file *arkFile) Name() string {
	return file.name
}
func (file *arkFile) Size() int64 {
	return file.size
}
func Filing(base, name, hash string, size int64) File {
	tttt := util.Extension(name)
	coding := Encoding(base, tttt, hash, size)
	return NewFile(coding.Code(), hash, name, size)
}

func Uploading(file string) Map {
	stat, err := os.Stat(file)
	if err != nil {
		return nil
	}

	hash := util.Sha1File(file)
	if hash == "" {
		return nil
	}
	filename := stat.Name()
	extension := util.Extension(file)
	mimetype := mBase.Mimetype(extension)
	length := stat.Size()

	return Map{
		"hash":      hash,
		"filename":  filename,
		"extension": extension,
		"mimetype":  mimetype,
		"length":    length,
		"tempfile":  file,
	}
}

//如果hash过长，就直接放在前面
//HASH短，就参与加密
func Encoding(base, tttt, hash string, size int64) *Coding {
	//tttt := util.Extension(name)	//扩展名
	return &Coding{
		base, tttt, hash, size,
	}

	//if len(hash) < 32 {
	//	return Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", base, tttt, hash, size))
	//}
	//return hash + "$" + Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", base, tttt, "", size))
}
func Decoding(code string) *Coding {
	codes := strings.Split(code, "$")
	hash := ""
	if len(codes) > 1 {
		hash = codes[0]
		code = codes[1]
	}

	str := Decrypt(code)
	if str == "" {
		return nil
	}

	//兼容老的file，coding，解析
	if strings.Contains(str, "\n") {
		//s\nnode\ntype\nhash
		args := strings.Split(str, "\n")
		if len(args) != 4 {
			return nil
		}
		return &Coding{
			Base: "", //node 直接base为空了，因为不用处理节点，file模块只服务单节点
			Type: args[2],
			Hash: args[3],
			Size: 0,
		}
	} else {
		args := strings.Split(str, "\t")
		if len(args) != 4 {
			return nil
		}

		if vv, err := strconv.ParseInt(args[3], 10, 64); err != nil {
			return nil
		} else {
			coding := &Coding{Base: args[0], Type: args[1], Hash: args[2], Size: vv}
			if hash != "" {
				coding.Hash = hash
			}

			return coding
		}
	}
}

//func (module *fileModule) Encode(node, tttt, hash string) string {
//	return Encrypt(fmt.Sprintf("s\n%s\n%s\n%s", node, tttt, hash))
//}
//
//func (module *fileModule) Decode(code string) *FileCoding {
//	str := Decrypt(code)
//	if str == "" {
//		return nil
//	}
//	args := strings.Split(str, "\n")
//	if len(args) != 4 {
//		return nil
//	}
//
//	return &FileCoding{
//		Node: args[1],
//		Type: args[2],
//		Hash: args[3],
//	}
//}

func Browse(code, name string, expires ...time.Duration) string {
	return safeBrowse(code, name, "", "", expires...)
}
func safeBrowse(code string, name string, id, ip string, expires ...time.Duration) string {

	//判断一下，是不是，要丢到存储层去处理
	coding := Decoding(code)
	if coding == nil {
		return "[error code]"
	}
	if coding.isStore() {
		if cfg, ok := Config.Store[coding.Base]; ok {
			if cfg.Browse {
				return Store(coding.Base).Browse(code, name, expires...)
			}
		}
	}

	expiry := time.Hour * 24
	if Config.File.Expiry != "" {
		if vv, err := util.ParseDuration(Config.File.Expiry); err == nil {
			expiry = vv
		}
	}
	if len(expires) > 0 {
		expiry = expires[0]
	}

	//expiry, session, address
	deadline := time.Now().Add(expiry).Unix()
	if expiry <= 0 {
		deadline = 0
	} else {
		//取5余数再补5秒，保证统一在5-10秒的时间戳
		//因为，如果有多个记录同一文件时候，会生成不同的时间戳
		//会被客户端认为是不同的文件，重复请求
		mod := deadline % 5
		deadline += (5 - mod) + 5
	}
	tokens := []int64{
		BrowseToken, deadline, Dehash(id), util.Ip2Num(ip),
	}

	token := Enhashs(tokens)

	ext := "x"
	if coding.Type != "" {
		ext = coding.Type
	}

	return Route("file.browse", Map{
		"{code}": code, "{ext}": ext, "token": token, "name": name,
	})

}
func Preview(code string, w, h, t int64, id, ip string, expires ...time.Duration) string {
	return safePreview(code, w, h, t, "", "", expires...)
}
func safePreview(code string, w, h, t int64, id, ip string, expires ...time.Duration) string {

	//判断一下，是不是，要丢到存储层去处理
	coding := Decoding(code)
	if coding == nil {
		return "[error code]"
	}
	if coding.isStore() {
		if cfg, ok := Config.Store[coding.Base]; ok {
			if cfg.Preview {
				return Store(coding.Base).Preview(code, w, h, t, expires...)
			}
		}
	}

	expiry := time.Hour * 24
	if Config.File.Expiry != "" {
		if vv, err := util.ParseDuration(Config.File.Expiry); err == nil {
			expiry = vv
		}
	}
	if len(expires) > 0 {
		expiry = expires[0]
	}

	//expiry, session, address
	deadline := time.Now().Add(expiry).Unix()
	if expiry <= 0 {
		deadline = 0
	} else {
		//取5余数再补5秒，保证统一在5-10秒的时间戳
		//因为，如果有多个记录同一文件时候，会生成不同的时间戳
		//会被客户端认为是不同的文件，重复请求
		mod := deadline % 5
		deadline += (5 - mod) + 5
	}

	tokens := []int64{
		PreviewToken, deadline, Dehash(id), util.Ip2Num(ip),
	}
	token := Enhashs(tokens)

	//ext := "x"
	//if coding.Type != "" {
	//	ext = coding.Type
	//}

	return Route("file.preview", Map{
		"{code}": code, "{size}": []int64{w, h, t}, "{ext}": "jpg", "token": token,
	})
}
