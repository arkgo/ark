package ark

import (
	. "github.com/arkgo/base"
)

type (
//arkResult struct {
//	code  int
//	error string
//	args  []Any
//}
)

var (
	OK, Fail, Found, Retry, Invalid *Res
)

func newResult(error string, args ...Any) *Res {
	return &Res{-1, error, args}
}
func codeResult(code int, error string, args ...Any) *Res {
	return &Res{code, error, args}
}
func errResult(err error) *Res {
	return &Res{-1, err.Error(), []Any{}}
}

func Result(code int, state string, text string, overrides ...bool) *Res {
	if len(overrides) == 0 {
		overrides = append(overrides, false) //默认不替换
	}
	ark.Basic.State(state, State{Code: code, String: text}, overrides...)
	return codeResult(code, state) //结束不包括使用的文字，需要文字的时候走basic.String方法拿
}

//func (res *arkResult) Code() int {
//	if res == nil {
//		return 0
//	}
//	return mBase.Coding(res.error)
//}
//func (res *arkResult) Error() string {
//	if res == nil {
//		return "ok"
//	}
//	return res.error
//}
//func (res *arkResult) Args() []Any {
//	return res.args
//}
//
//func (res *arkResult) OK() bool {
//	if res == nil {
//		return true
//	}
//	return res.Code() == 0
//}
//func (res *arkResult) Fail() bool {
//	return false == res.OK()
//}
