package ark

import (
	"time"

	. "github.com/arkgo/asset"
	"github.com/arkgo/asset/util"
)

func builtin() {
	built_router()
}

func built_router() {

	browse := ark.Config.File.Site + "." + "browse"
	preview := ark.Config.File.Site + "." + "preview"

	Register(browse, Router{
		Uri: "/{code}.{ext}",
		Routing: Routing{
			GET: Router{
				Name: "浏览文件", Desc: "浏览文件",
				Args: Vars{
					"code": Var{
						Type: "string", Required: true, Name: "文件编码", Desc: "文件编码",
					},
					"token": Var{
						Type: "[int]", Required: true, Name: "令牌", Desc: "令牌",
						Encode: "digits", Decode: "digits",
					},
					"name": Var{
						Type: "string", Required: false, Name: "自定义文件名", Desc: "自定义文件名",
					},
				},
				Action: func(ctx *Http) {

					code := ctx.Args["code"].(string)
					//data := Decoding(code)
					//if data == nil {
					//	ctx.Text("无效访问")
					//	return
					//}

					tokens := ctx.Args["token"].([]int64)
					if len(tokens) < 4 {
						ctx.Text("无效访问令牌1")
						return
					}

					token, expiry, session, address := tokens[0], tokens[1], tokens[2], tokens[3]
					if token != BROWSE_TOKEN {
						ctx.Text("无效访问令牌2")
						return
					}

					if expiry > 0 && ExpiryTokenized() && expiry < time.Now().Unix() {
						//超时不让访问了
						ctx.Text("无效访问令牌3")
						return
					}
					if session > 0 && SessionTokenized() && ctx.Id != Enhash(session) {
						ctx.Text("无效访问令牌4")
						return
					}
					if address > 0 && AddressTokenized() && ctx.Ip() != util.Num2Ip(address) {
						ctx.Text("无效访问令牌5")
						return
					}

					//用代码来跳转试一下
					if name, ok := ctx.Args["name"].(string); ok && name != "" {
						ctx.Remote(code, name)
					} else {
						ctx.Remote(code)
					}
				},
			},
		},
	})

	//自带一个文件浏览和预览的路由
	Register(preview, Router{
		Uri:  "/{code}/{size}.{ext}",
		Name: "预览文件", Desc: "预览文件",
		Args: Vars{
			"code": Var{
				Type: "string", Required: true, Name: "文件编码", Desc: "文件编码",
			},
			"size": Var{
				Type: "[int]", Required: true, Name: "文件编码", Desc: "文件编码",
				Encode: "digits", Decode: "digits",
			},
			"token": Var{
				Type: "[int]", Required: true, Name: "令牌", Desc: "令牌",
				Encode: "digits", Decode: "digits",
			},
		},
		Action: func(ctx *Http) {

			//url1 := ctx.Url.Browse("QmcB55KNpU1E8uvqFtFa9QTFWPTHnfSmC1N7Hg6c5qYYX9$kJ1qlvQxjJbuKZrC2HbveH0vd3hy")
			//Debug("url1", url1)
			//url2 := ctx.Url.Preview("Qmbmg6rDfS81CBpsmaq4nfqTK21Joayurh1n2S9C61BF3C$kJ1qlvQslalC2H4Wd3j5df==", 256, 0, 0)
			//Debug("url", url2)

			code := ctx.Args["code"].(string)
			//data := Decoding(code)
			//if data == nil {
			//	ctx.Text("无效访问代码")
			//	return
			//}

			size := ctx.Args["size"].([]int64)
			if len(size) != 3 {
				ctx.Text("无效访问参数")
				return
			}

			tokens := ctx.Args["token"].([]int64)
			if len(tokens) < 4 {
				ctx.Text("无效访问令牌1")
				return
			}

			token, expiry, session, address := tokens[0], tokens[1], tokens[2], tokens[3]
			if token != PREVIEW_TOKEN {
				ctx.Text("无效访问令牌2")
				return
			}

			if expiry > 0 && ExpiryTokenized() && expiry < time.Now().Unix() {
				//超时不让访问了
				ctx.Text("无效访问令牌3")
				return
			}
			if session > 0 && SessionTokenized() && ctx.Id != Enhash(session) {
				ctx.Text("无效访问令牌4")
				return
			}
			if address > 0 && AddressTokenized() && ctx.Ip() != util.Num2Ip(address) {
				// ark.Logger.Warning("无效访问令牌5", ctx.Ip(), util.Num2Ip(address))
				ctx.Text("无效访问令牌5")
				return
			}

			ctx.Thumbnail(code, size[0], size[1], size[2])
		},
	})

}
