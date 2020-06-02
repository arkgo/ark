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

	. "github.com/arkgo/asset"
	"github.com/arkgo/asset/hashring"
	"github.com/arkgo/asset/util"
	"github.com/disintegration/imaging"
)

type (
	FileConfig struct {
		Sharding  int      `toml:"sharding"`
		Storage   string   `toml:"storage"`
		Thumbnail string   `toml:"thumbnail"`
		Cache     string   `toml:"cache"`
		Expiry    string   `toml:"expiry"`
		Site      string   `toml:"site"`
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

		Upload(target string, metadata Map) (File, Files, error)
		Download(file File) (string, error)
		Remove(file File) error

		Browse(file File, name string, expiries ...time.Duration) (string, error)
		Preview(file File, w, h, t int64, expiries ...time.Duration) (string, error)
	}
	StoreHealth struct {
		Workload int64
	}

	File interface {
		Code() string
		Hash() string
		Name() string
		Type() string
		Size() int64
	}
	Files []File

	storeFile struct {
		code string
		conn string
		hash string
		name string
		tttt string
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
func (module *storeModule) getConnect(names ...string) StoreConnect {
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
		return connect
	}

	return nil
}

//上传文件，是不是随便选一个库，还是选第一个库
func (module *storeModule) Upload(target string, metadata Map, bases ...string) (File, Files, error) {
	conn := module.getConnect(bases...)
	if conn == nil {
		return nil, nil, errors.New("无效连接")
	}
	return conn.Upload(target, metadata)
}

//下载文件，集成file和store
func (module *storeModule) Download(code string) (string, error) {
	file := module.Decode(code)
	if file == nil {
		return "", errors.New("解码失败")
	}

	if file.conn != "" {
		conn := module.getConnect(file.conn)
		if conn == nil {
			return "", errors.New("无效连接")
		}
		return conn.Download(file)
	} else {
		_, _, sFile, err := module.storaging(file)
		if err != nil {
			return "", err
		}
		return sFile, nil
	}
}

func (module *storeModule) Remove(code string) error {
	data := module.Decode(code)
	if data == nil {
		return errors.New("无效数据")
	}

	if data.conn != "" {
		conn := module.getConnect(data.conn)
		if conn == nil {
			return errors.New("无效连接")
		}
		return conn.Remove(data)
	} else {
		_, _, sFile, err := module.storaging(data)
		if err != nil {
			return err
		}
		return os.Remove(sFile)
	}

}

//保存文件到 file, 而不是store
func (module *storeModule) Storage(target string) (File, Files, error) {
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

		files := Files{}
		for _, file := range dirs {
			if !file.IsDir() {

				source := path.Join(target, file.Name())
				hash := util.Sha1File(source)
				if hash == "" {
					return nil, nil, errors.New("hash error")
				}

				file := module.Filing("", hash, file.Name(), file.Size())

				err := module.storage(source, file)
				if err != nil {
					return nil, nil, err
				}

				//file := module.NewFile(coding.Coding(), coding.Hash, file.Name(), file.Size())
				files = append(files, file)
			}
		}

		return nil, files, nil

	} else {

		hash := util.Sha1File(target)
		if hash == "" {
			return nil, nil, errors.New("hash error")
		}

		file := module.Filing("", hash, stat.Name(), stat.Size())

		err := module.storage(target, file)
		if err != nil {
			return nil, nil, err
		}

		// file := module.NewFile(coding.Coding(), coding.Hash, stat.Name(), stat.Size())
		return file, nil, nil
	}

}

//file存储文件
func (module *storeModule) storage(source string, coding *storeFile) error {
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
func (module *storeModule) thumbnail(code string, w, h, t int64) (string, File, error) {
	data := module.Decode(code)
	if data == nil {
		return "", nil, errors.New("error code")
	}
	if data.isimage() == false {
		//非图片不处理缩图
		return "", nil, errors.New("not image")
	}

	//先获取缩略图的文件
	_, _, tfile, err := module.thumbnailing(data, w, h, t)
	if err != nil {
		Debug("生成缩图获取保存位置", err, code)
		return "", nil, nil
	}

	//如果缩图已经存在，直接返回
	_, err = os.Stat(tfile)
	if err == nil {
		return tfile, data, nil
	}

	//这里要处理，是file，或是 store，获取原文件不一样
	sfile := ""
	if data.stored() {
		conn := module.getConnect(data.conn)
		if conn == nil {
			return "", nil, errors.New("无效连接")
		}
		fff, err := conn.Download(data)
		if err != nil {
			Warning("生成缩图下载文件", err, code)
			return "", nil, err
		} else {
			sfile = fff
		}
	} else {
		//获取存储的文件
		_, _, fff, err := module.storaging(data)
		if err != nil {
			Warning("生成缩图获取文件", err, code)
			return "", nil, err
		} else {
			sfile = fff
		}
	}

	sf, err := os.Open(sfile)
	if err != nil {
		Warning("生成缩图打开文件", err, code)
		return "", nil, err
	}
	defer sf.Close()

	cfg, err := util.DecodeImageConfig(sf)
	if err != nil {
		Warning("生成缩图解析图片配置", err, code)
		return "", nil, err
	}

	//流重新定位
	sf.Seek(0, 0)
	img, err := imaging.Decode(sf)
	if err != nil {
		Warning("生成缩图解析图片", err, code)
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
		Warning("生成缩图保存文件", err, code)
		return "", nil, err
	}

	//更新不打开文件，直接返回文件路径
	return tfile, data, nil
}

//获取file的存储路径信息
func (module *storeModule) storaging(data *storeFile) (string, string, string, error) {
	if ring := module.hashring.Locate(data.hash); ring != "" {

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
func (module *storeModule) thumbnailing(data *storeFile, width, height, tttt int64) (string, string, string, error) {
	if ring := module.hashring.Locate(data.hash); ring != "" {

		// data.Type = "jpg"	//不能直接改，因为是*data，所以扩展名不同的，生成缩图就有问题了，ring变了
		//namenoext := strings.TrimSuffix(data.Name, "."+data.Type)

		ext := "jpg"
		if data.tttt != "" {
			ext = data.tttt
		}

		tpath := path.Join(ark.Config.File.Thumbnail, ring, data.hash)
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

// func (sc *storeFile) Coding() string {
// 	if sc.code != "" {
// 		sc.code = Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", sc.Base, sc.Type, sc.Hash, sc.Size))
// 	}
// 	return sc.code
// }

func (sc *storeFile) Mimetype() string {
	if sc != nil {
		return ark.Basic.Mimetype(sc.tttt)
	}
	return ""
}
func (sc *storeFile) Fullname() string {
	if sc != nil {
		if sc.tttt != "" {
			return fmt.Sprintf("%s.%s", sc.hash, sc.tttt)
		}
		return sc.hash
	}
	return ""
}

//func (sc *storeFile) Node() int {
//	if vv,err := strconv.ParseInt(sc.Base, 10, 32); err == nil {
//		return int(vv)
//	}
//	return 0
//}

// func (sc *storeFile) isFile() bool {
// 	return sc.Base == ""
// }
// func (sc *storeFile) isStore() bool {
// 	return sc.Base != ""
// }

func (sc *storeFile) stored() bool {
	return sc.conn != ""
}
func (sc *storeFile) isimage() bool {
	return sc.tttt == "jpg" ||
		sc.tttt == "jpeg" ||
		sc.tttt == "png" ||
		sc.tttt == "gif" ||
		sc.tttt == "bmp"
}
func (sc *storeFile) isvideo() bool {
	return sc.tttt == "mp4" ||
		sc.tttt == "mkv" ||
		sc.tttt == "wmv" ||
		sc.tttt == "ts" ||
		sc.tttt == "mpeg" ||
		sc.tttt == "ts"
}
func (sc *storeFile) isaudio() bool {
	return sc.tttt == "mp3" ||
		sc.tttt == "wma" ||
		sc.tttt == "wav"
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
func (sf *storeFile) Type() string {
	return sf.tttt
}
func (sf *storeFile) Size() int64 {
	return sf.size
}

/* storeFile 实体 end */

// func (module *storeModule) NewFile(code, hash, name string, size int64) File {
// 	return &storeFile{code, hash, name, size}
// }
func (module *storeModule) Filing(conn, hash, name string, size int64) *storeFile {
	file := &storeFile{}
	file.conn = conn
	file.hash = hash
	file.name = name
	file.tttt = util.Extension(name)
	file.size = size
	file.code = module.Encode(file)

	return file
}

func (module *storeModule) Stat(file string) Map {
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

func (module *storeModule) Encode(file *storeFile) string {
	return Encrypt(fmt.Sprintf("%s\t%s\t%s\t%d", file.conn, file.hash, file.tttt, file.size))
}
func (module *storeModule) Decode(code string) *storeFile {
	str := ark.Codec.Decrypt(code)
	if str == "" {
		return nil
	}

	args := strings.Split(str, "\t")
	if len(args) != 4 {
		return nil
	}

	file := &storeFile{}
	file.conn = args[0]
	file.hash = args[1]
	file.tttt = args[2]
	file.size = 0
	if vv, err := strconv.ParseInt(args[3], 10, 64); err == nil {
		file.size = vv
	}

	file.code = code

	return file
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
	if coding.conn != "" {
		//使用远程访问
		if cfg, ok := ark.Config.Store[coding.conn]; ok {
			if cfg.Browse {
				conn := module.getConnect(coding.conn)
				if conn == nil {
					return ""
				}
				url, _ := conn.Browse(coding, name, expires...)
				return url
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

	tokens := []int64{
		BROWSE_TOKEN, deadline, Dehash(id), util.Ip2Num(ip),
	}

	token := Enhashs(tokens)

	ext := "x"
	if coding.Type() != "" {
		ext = coding.Type()
	}

	browse := ark.Config.File.Site + "." + "browse"

	return Route(browse, Map{
		"{code}": code, "{ext}": ext, "token": token, "name": name,
	})

}
func (module *storeModule) Preview(code string, w, h, t int64, expires ...time.Duration) string {
	return module.safePreview(code, w, h, t, "", "", expires...)
}
func (module *storeModule) safePreview(code string, w, h, t int64, id, ip string, expires ...time.Duration) string {

	//判断一下，是不是，要丢到存储层去处理
	coding := module.Decode(code)
	if coding == nil {
		return code
	}
	if coding.conn != "" {
		if cfg, ok := ark.Config.Store[coding.conn]; ok {
			if cfg.Preview {
				conn := module.getConnect(coding.conn)
				if conn == nil {
					return ""
				}
				url, _ := conn.Preview(coding, w, h, t, expires...)
				return url
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

	tokens := []int64{
		PREVIEW_TOKEN, deadline, Dehash(id), util.Ip2Num(ip),
	}
	token := Enhashs(tokens)

	// ext := "x"
	// if coding.Type != "" {
	// 	ext = coding.Type
	// }

	preview := ark.Config.File.Site + "." + "preview"

	return Route(preview, Map{
		"{code}": code, "{size}": []int64{w, h, t}, "{ext}": "jpg", "token": token,
	})
}

//生成文件信息，给驱动用的
func NewFile(conn, hash, name string, size int64) File {
	return ark.Store.Filing(conn, hash, name, size)
}
func StatFile(file string) Map {
	return ark.Store.Stat(file)
}

func Storage(target string) (File, Files, error) {
	return ark.Store.Storage(target)
}
func Remove(code string) error {
	return ark.Store.Remove(code)
}

func Browse(code, name string, expires ...time.Duration) string {
	return ark.Store.Browse(code, name, expires...)
}
func Preview(code string, w, h, t int64, expires ...time.Duration) string {
	return ark.Store.Preview(code, w, h, t, expires...)
}

func Upload(target string, metadata Map, bases ...string) (File, Files, error) {
	return ark.Store.Upload(target, metadata, bases...)
}
func Download(code string) (string, error) {
	return ark.Store.Download(code)
}

func SessionTokenized() bool {
	for _, s := range ark.Config.File.Tokens {
		if s == "session" {
			return true
		}
	}
	return false
}
func AddressTokenized() bool {
	for _, s := range ark.Config.File.Tokens {
		if s == "address" {
			return true
		}
	}
	return false
}
func ExpiryTokenized() bool {
	for _, s := range ark.Config.File.Tokens {
		if s == "expiry" {
			return true
		}
	}
	return false
}
