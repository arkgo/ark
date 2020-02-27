package ark

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/arkgo/asset/hashring"
	"github.com/arkgo/asset/util"
	"github.com/disintegration/imaging"
)

type (
	FileConfig struct {
		Expiry    string   `toml:"expiry"`
		Sharding  int      `toml:"sharding"`
		Storage   string   `toml:"storage"`
		Thumbnail string   `toml:"thumbnail"`
		Cache     string   `toml:"cache"`
		Tokens    []string `toml:"tokens"`
		//Hosts		[]string	`toml:"hosts"`
	}
	fileModule struct {
		hashring *hashring.HashRing
	}
)

func (module *fileModule) Upload(target string) (File, Files) {
	stat, err := os.Stat(target)
	if err != nil {
		return nil, nil
	}

	//是目录，就整个目录上传
	if stat.IsDir() {

		dirs, err := ioutil.ReadDir(target)
		if err != nil {
			return nil, nil
		}

		files := Files{}
		for _, file := range dirs {
			if !file.IsDir() {

				source := path.Join(target, file.Name())
				hash := util.Sha1File(source)
				if hash == "" {
					return nil, nil
				}

				ext := util.Extension(file.Name())

				coding := Encoding("", ext, hash, file.Size())
				err := module.Storage(source, coding)
				if err != nil {
					return nil, nil
				}

				file := NewFile(coding.Code(), coding.Hash, file.Name(), file.Size())
				files = append(files, file)
			}
		}

		return nil, files

	} else {

		hash := util.Sha1File(target)
		if hash == "" {
			return nil, nil
		}

		ext := util.Extension(stat.Name())
		coding := Encoding("", ext, hash, stat.Size())
		err := module.Storage(target, coding)
		if err != nil {
			return nil, nil
		}

		file := NewFile(coding.Code(), coding.Hash, stat.Name(), stat.Size())
		return file, nil
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

//下载文件，因为存储是本地，所以直接返回路径就可以
//不过，如果是多节点的话，肯定获取不到其它其它的文件，还需要做更多处理
//算了吧，File模块就是本地存储，不处理多节点，多节点就上store模块，写驱动
func (module *fileModule) Download(code string) string {
	data := Decoding(code)
	_, _, sFile, err := module.storaging(data)
	if err != nil {
		return ""
	}
	return sFile
}

//存文件
func (module *fileModule) Storage(source string, coding *Coding) error {
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

//获取缩图路径，注意判断一下节点，
//非本节点的文件，应该直接失败
func (module *fileModule) Thumbnail(code string, w, h, t int64) (string, *Coding, error) {
	data := Decoding(code)
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
		sb := mStore.Base(data.Base)
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

func (module *fileModule) Remove(code string) error {
	data := Decoding(code)
	if data == nil {
		return errors.New("无效数据")
	}

	_, _, sFile, err := module.storaging(data)
	if err != nil {
		return err
	}

	err = os.Remove(sFile)
	if err != nil {
		return err
	}

	return nil
}

//初始化
func (module *fileModule) initing() {
	rings := map[string]int{}
	for i := 1; i <= Config.File.Sharding; i++ {
		rings[fmt.Sprintf("%v", i)] = 1
	}
	module.hashring = hashring.New(rings)
}

//退出
func (module *fileModule) exiting() {
	//删除hashring，不需要
}

func (module *fileModule) storaging(data *Coding) (string, string, string, error) {
	if ring := module.hashring.Locate(data.Hash); ring != "" {

		spath := path.Join(Config.File.Storage, ring)
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

func (module *fileModule) thumbnailing(data *Coding, width, height, tttt int64) (string, string, string, error) {
	if ring := module.hashring.Locate(data.Hash); ring != "" {

		// data.Type = "jpg"	//不能直接改，因为是*data，所以扩展名不同的，生成缩图就有问题了，ring变了
		//namenoext := strings.TrimSuffix(data.Name, "."+data.Type)

		ext := "jpg"
		if data.Type != "" {
			ext = data.Type
		}

		tpath := path.Join(Config.File.Thumbnail, ring, data.Hash)
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
