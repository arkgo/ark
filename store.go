package ark

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arkgo/asset/hashring"
	"github.com/arkgo/asset/util"
	. "github.com/arkgo/base"
	"github.com/disintegration/imaging"
)

type (
	FileConfig struct {
		Sharding  int      `toml:"sharding"`
		Storage   string   `toml:"storage"`
		Thumbnail string   `toml:"thumbnail"`
		Cache     string   `toml:"cache"`
		Expiry    string   `toml:"expiry"`
		Tokens    []string `toml:"tokens"`
		//Hosts		[]string	`toml:"hosts"`
	}
	StoreConfig struct {
		Driver  string `toml:"driver"`
		Weight  int    `toml:"weight"`
		Cache   string `toml:"cache"`
		Browse  bool   `toml:"browse"`
		Preview bool   `toml:"preview"`
		Setting Map    `toml:"setting"`
	}
	StoreDriver interface {
		Connect(name string, config StoreConfig) (StoreConnect, error)
	}
	StoreConnect interface {
		Open() error
		Health() (StoreHealth, error)
		Close() error

		Base() StoreBase
	}
	StoreHealth struct {
		Workload int64
	}
	StoreBase interface {
		Close() error
		Erred() error

		Upload(target string, metadatas ...Map) (StoreFile, StoreFiles, error)
		Download(code string) string
		Remove(code string) error

		Browse(code string, name string, expiry ...time.Duration) string
		Preview(code string, w, h, t int64, expiry ...time.Duration) string
	}

	StoreCode struct {
		code string

		Base string
		Type string
		Hash string
		Size int64
	}
	StoreFile interface {
		Code() string
		Hash() string
		Name() string
		Size() int64
	}
	StoreFiles []StoreFile

	storeFile struct {
		code string
		hash string
		name string
		size int64
	}
)

type (
	storeModule struct {
		mutex    sync.Mutex
		drivers  map[string]StoreDriver
		connects map[string]StoreConnect
		hashring *hashring.HashRing
	}
)

func newStore() *storeModule {
	return &storeModule{
		drivers:  make(map[string]StoreDriver, 0),
		connects: make(map[string]StoreConnect, 0),
	}
}

func (module *storeModule) Driver(name string, driver StoreDriver, overrides ...bool) {
	if driver == nil {
		panic("[存储]驱动不可为空")
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

func (module *storeModule) connecting(name string, config StoreConfig) (StoreConnect, error) {
	if driver, ok := module.drivers[config.Driver]; ok {
		return driver.Connect(name, config)
	}
	panic("[存储]不支持的驱动：" + config.Driver)
}

//初始化
func (module *storeModule) initing() {
	rings := map[string]int{}
	for i := 1; i <= ark.Config.File.Sharding; i++ {
		rings[fmt.Sprintf("%v", i)] = 1
	}
	module.hashring = hashring.New(rings)

	for name, config := range ark.Config.Store {
		//连接
		connect, err := module.connecting(name, config)
		if err != nil {
			panic("[存储]连接失败：" + err.Error())
		}

		//打开连接
		err = connect.Open()
		if err != nil {
			panic("[存储]打开失败：" + err.Error())
		}

		module.connects[name] = connect
	}
}

//退出
func (module *storeModule) exiting() {
	for _, connect := range module.connects {
		connect.Close()
	}
}

//返回文件Base对象
func (module *storeModule) Base(names ...string) StoreBase {
	name := DEFAULT
	if len(names) > 0 {
		name = names[0]
	} else {
		for key, _ := range module.connects {
			name = key
			break
		}
	}

	if connect, ok := module.connects[name]; ok {
		return connect.Base()
	}
	panic("[存储]无效存储连接")
}

//上传文件，是不是随便选一个库，还是选第一个库
func (module *storeModule) Upload(target string, metadata Map, bases ...string) (StoreFile, StoreFiles, error) {
	sb := module.Base(bases...)
	return sb.Upload(target, metadata)
}

//下载文件，集成file和store
func (module *storeModule) Download(code string) string {
	coding := module.Decode(code)
	if coding == nil {
		return ""
	}

	if coding.isStore() {
		sb := module.Base(coding.Base)
		return sb.Download(code)
	} else {
		_, _, sFile, err := module.storaging(coding)
		if err == nil {
			return sFile
		}
	}
	return ""
}

func (module *storeModule) Remove(code string) error {
	data := module.Decode(code)
	if data == nil {
		return errors.New("无效数据")
	}

	if data.isStore() {
		sb := module.Base(data.Base)
		return sb.Remove(code)
	} else {
		_, _, sFile, err := module.storaging(data)
		if err != nil {
			return err
		}
		return os.Remove(sFile)
	}

}

//保存文件到 file, 而不是store
func (module *storeModule) Save(target string) (StoreFile, StoreFiles, error) {
	stat, err := os.Stat(target)
	if err != nil {
		return nil, nil, err
	}

	//是目录，就整个目录上传
	if stat.IsDir() {

		dirs, err := ioutil.ReadDir(target)
		if err != nil {
			return nil, nil, err
		}

		files := StoreFiles{}
		for _, file := range dirs {
			if !file.IsDir() {

				source := path.Join(target, file.Name())
				hash := util.Sha1File(source)
				if hash == "" {
					return nil, nil, errors.New("hash error")
				}

				ext := util.Extension(file.Name())

				coding := module.Encode("", ext, hash, file.Size())
				err := module.Storage(source, coding)
				if err != nil {
					return nil, nil, err
				}

				file := module.NewFile(coding.Coding(), coding.Hash, file.Name(), file.Size())
				files = append(files, file)
			}
		}

		return nil, files, nil

	} else {

		hash := util.Sha1File(target)
		if hash == "" {
			return nil, nil, errors.New("hash error")
		}

		ext := util.Extension(stat.Name())
		coding := module.Encode("", ext, hash, stat.Size())
		err := module.Storage(target, coding)
		if err != nil {
			return nil, nil, err
		}

		file := module.NewFile(coding.Coding(), coding.Hash, stat.Name(), stat.Size())
		return file, nil, nil
	}

	//data := &Coding{
	//	fmt.Sprintf("%v", Config.Node.Id),
	//	fmt.Sprintf("%v", file["extension"]),
	//	fmt.Sprintf("%v", file["hash"]),
	//	file["length"].(int64),
	//}

	//code := Encoding(data.Base, data.Type, data.Hash, data.Size)
	//
	//_, _, sFile, err := module.storaging(data)
	//if err != nil {
	//	return "", err
	//}
	//
	////如果文件已经存在
	//if _,err := os.Stat(sFile); err == nil {
	//	return code, nil
	//}
	//
	////打开上传的文件
	//tempfile := file["tempfile"].(string)
	//fff, err := os.Open(tempfile)
	//if err != nil {
	//	return "", err
	//}
	//defer fff.Close()
	//
	////创建文件
	//save, err := os.OpenFile(sFile, os.O_WRONLY|os.O_CREATE, 0777)
	//if err != nil {
	//	return "", err
	//}
	//defer save.Close()
	//
	////复制文件
	//_, err = io.Copy(save, fff)
	//if err != nil {
	//	return "", err
	//}
	//
	//return code, nil
}

//file存储文件
func (module *storeModule) Storage(source string, coding *StoreCode) error {
	_, _, sFile, err := module.storaging(coding)
	if err != nil {
		return err
	}

	//如果文件已经存在，直接返回
	if _, err := os.Stat(sFile); err == nil {
		return nil
	}

	//打开原始文件
	fff, err := os.Open(source)
	if err != nil {
		return err
	}
	defer fff.Close()

	//创建文件
	save, err := os.OpenFile(sFile, os.O_WRONLY|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer save.Close()

	//复制文件
	_, err = io.Copy(save, fff)
	if err != nil {
		return err
	}

	return nil
}

// file生成获取缩图并返回路径，
// 待优化要不要判断一下节点，？
func (module *storeModule) Thumbnail(code string, w, h, t int64) (string, *StoreCode, error) {
	data := module.Decode(code)
	if data == nil {
		return "", nil, errors.New("error code")
	}
	if data.IsImage() == false {
		//非图片不处理缩图
		return "", nil, errors.New("not image")
	}

	//先获取缩略图的文件
	_, _, tfile, err := module.thumbnailing(data, w, h, t)
	if err != nil {
		return "", nil, nil
	}

	//如果缩图已经存在，直接返回
	_, err = os.Stat(tfile)
	if err == nil {
		return tfile, data, nil
	}

	//这里要处理，是file，或是 store，获取原文件不一样
	sfile := ""
	if data.isFile() {
		//获取存储的文件
		_, _, fff, err := module.storaging(data)
		if err != nil {
			return "", nil, err
		} else {
			sfile = fff
		}
	} else {
		sb := module.Base(data.Base)
		fff := sb.Download(code)
		if err := sb.Erred(); err != nil {
			return "", nil, err
		} else {
			sfile = fff
		}
	}

	sf, err := os.Open(sfile)
	if err != nil {
		return "", nil, err
	}
	defer sf.Close()

	cfg, err := util.DecodeImageConfig(sf)
	if err != nil {
		return "", nil, err
	}

	//流重新定位
	sf.Seek(0, 0)
	img, err := imaging.Decode(sf)
	if err != nil {
		return "", nil, err
	}

	//计算新度和新高
	ratio := float64(cfg.Width) / float64(cfg.Height)
	width, height := float64(w), float64(h)
	if width == 0 {
		width = height * ratio
	} else if height == 0 {
		height = width / ratio
	}

	thumb := imaging.Thumbnail(img, int(width), int(height), imaging.NearestNeighbor)
	err = imaging.Save(thumb, tfile)
	if err != nil {
		return "", nil, err
	}

	//更新不打开文件，直接返回文件路径
	return tfile, data, nil
}

//获取file的存储路径信息
func (module *storeModule) storaging(data *StoreCode) (string, string, string, error) {
	if ring := module.hashring.Locate(data.Hash); ring != "" {

		spath := path.Join(ark.Config.File.Storage, ring)
		sfile := path.Join(spath, data.Fullname())

		// //创建目录
		err := os.MkdirAll(spath, 0777)
		if err != nil {
			return "", "", "", errors.New("生成目录失败")
		}

		return ring, spath, sfile, nil
	}

	return "", "", "", errors.New("配置异常")
}

//获取file的缩图路径信息
func (module *storeModule) thumbnailing(data *StoreCode, width, height, tttt int64) (string, string, string, error) {
	if ring := module.hashring.Locate(data.Hash); ring != "" {

		// data.Type = "jpg"	//不能直接改，因为是*data，所以扩展名不同的，生成缩图就有问题了，ring变了
		//namenoext := strings.TrimSuffix(data.Name, "."+data.Type)

		ext := "jpg"
		if data.Type != "" {
			ext = data.Type
		}

		tpath := path.Join(ark.Config.File.Thumbnail, ring, data.Hash)
		tname := fmt.Sprintf("%d-%d-%d.%s", width, height, tttt, ext)
		tfile := path.Join(tpath, tname)

		// //创建目录
		err := os.MkdirAll(tpath, 0777)
		if err != nil {
			return "", "", "", errors.New("生成目录失败")
		}

		return ring, tpath, tfile, nil
	}

	return "", "", "", errors.New("配置异常")
}

/* StoreCode 存储编码 begin */

func (sc *StoreCode) Coding() string {
	if sc.code != "" {
		sc.code = Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", sc.Base, sc.Type, sc.Hash, sc.Size))
	}
	return sc.code
}

func (sc *StoreCode) Mimetype() string {
	if sc != nil {
		return ark.Basic.Mimetype(sc.Type)
	}
	return ""
}
func (sc *StoreCode) Fullname() string {
	if sc != nil {
		if sc.Type != "" {
			return fmt.Sprintf("%s.%s", sc.Hash, sc.Type)
		}
		return sc.Hash
	}
	return ""
}

//func (sc *StoreCode) Node() int {
//	if vv,err := strconv.ParseInt(sc.Base, 10, 32); err == nil {
//		return int(vv)
//	}
//	return 0
//}

func (sc *StoreCode) isFile() bool {
	return sc.Base == ""
}
func (sc *StoreCode) isStore() bool {
	return sc.Base != ""
}

func (sc *StoreCode) IsImage() bool {
	return sc.Type == "jpg" ||
		sc.Type == "png" ||
		sc.Type == "gif" ||
		sc.Type == "bmp"
}
func (sc *StoreCode) IsVideo() bool {
	return sc.Type == "mp4" ||
		sc.Type == "mkv" ||
		sc.Type == "wmv" ||
		sc.Type == "ts" ||
		sc.Type == "mpeg" ||
		sc.Type == "ts"
}
func (sc *StoreCode) IsAudio() bool {
	return sc.Type == "mp3" ||
		sc.Type == "wma" ||
		sc.Type == "wav"
}

/* StoreCode 存储编码 end */

/* storeFile 实体 begin */

func (sf *storeFile) Code() string {
	return sf.code
}
func (sf *storeFile) Hash() string {
	return sf.hash
}
func (sf *storeFile) Name() string {
	return sf.name
}
func (sf *storeFile) Size() int64 {
	return sf.size
}

/* storeFile 实体 end */

func (module *storeModule) NewFile(code, hash, name string, size int64) StoreFile {
	return &storeFile{code, hash, name, size}
}
func (module *storeModule) Filing(base, name, hash string, size int64) StoreFile {
	tttt := util.Extension(name)
	coding := module.Encode(base, tttt, hash, size)
	return module.NewFile(coding.Coding(), hash, name, size)
}

func (module *storeModule) Uploading(file string) Map {
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
	mimetype := ark.Basic.Mimetype(extension)
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
func (module *storeModule) Encode(base, tttt, hash string, size int64) *StoreCode {
	//tttt := util.Extension(name)	//扩展名
	return &StoreCode{
		"", base, tttt, hash, size,
	}

	//if len(hash) < 32 {
	//	return Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", base, tttt, hash, size))
	//}
	//return hash + "$" + Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", base, tttt, "", size))
}
func (module *storeModule) Decode(code string) *StoreCode {
	str := ark.Serial.Decrypt(code)
	if str == "" {
		return nil
	}

	args := strings.Split(str, "\t")
	if len(args) != 4 {
		return nil
	}

	coding := &StoreCode{Base: args[0], Type: args[1], Hash: args[2], Size: 0}
	if vv, err := strconv.ParseInt(args[3], 10, 64); err == nil {
		coding.Size = vv
	}
	return coding
}

func (module *storeModule) Browse(code, name string, expires ...time.Duration) string {
	return module.safeBrowse(code, name, "", "", expires...)
}
func (module *storeModule) safeBrowse(code string, name string, id, ip string, expires ...time.Duration) string {

	//判断一下，是不是，要丢到存储层去处理
	coding := module.Decode(code)
	if coding == nil {
		return "[error code]"
	}
	if coding.isStore() {
		if cfg, ok := ark.Config.Store[coding.Base]; ok {
			if cfg.Browse {
				return module.Base(coding.Base).Browse(code, name, expires...)
			}
		}
	}

	expiry := time.Hour * 24
	if ark.Config.File.Expiry != "" {
		if vv, err := util.ParseDuration(ark.Config.File.Expiry); err == nil {
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
	// tokens := []int64{
	// 	0, deadline, Dehash(id), util.Ip2Num(ip),
	// }

	// token := Enhashs(tokens)

	// ext := "x"
	// if coding.Type != "" {
	// 	ext = coding.Type
	// }

	return ""

	// return Route("file.browse", Map{
	// 	"{code}": code, "{ext}": ext, "token": token, "name": name,
	// })

}
func (module *storeModule) Preview(code string, w, h, t int64, id, ip string, expires ...time.Duration) string {
	return module.safePreview(code, w, h, t, "", "", expires...)
}
func (module *storeModule) safePreview(code string, w, h, t int64, id, ip string, expires ...time.Duration) string {

	//判断一下，是不是，要丢到存储层去处理
	coding := module.Decode(code)
	if coding == nil {
		return "[error code]"
	}
	if coding.isStore() {
		if cfg, ok := ark.Config.Store[coding.Base]; ok {
			if cfg.Preview {
				return module.Base(coding.Base).Preview(code, w, h, t, expires...)
			}
		}
	}

	expiry := time.Hour * 24
	if ark.Config.File.Expiry != "" {
		if vv, err := util.ParseDuration(ark.Config.File.Expiry); err == nil {
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

	// tokens := []int64{
	// 	1, deadline, Dehash(id), util.Ip2Num(ip),
	// }
	// token := Enhashs(tokens)

	//ext := "x"
	//if coding.Type != "" {
	//	ext = coding.Type
	//}

	return ""

	// return Route("file.preview", Map{
	// 	"{code}": code, "{size}": []int64{w, h, t}, "{ext}": "jpg", "token": token,
	// })
}
