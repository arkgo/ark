package ark

import (
	"fmt"

	. "github.com/arkgo/asset"
)

type (
	SiteConfig struct {
		Name    string   `toml:"name"`
		Ssl     bool     `toml:"ssl"`
		Host    string   `toml:"host"`
		Hosts   []string `toml:"hosts"`
		Weights []int    `toml:"weights"`

		Charset string `toml:"charset"`
		Domain  string `toml:"domain"`
		Cookie  string `toml:"cookie"`
		Expiry  string `toml:"expiry"`
		MaxAge  string `toml:"maxage"`

		Encode  string `toml:"encode"`
		Decode  string `toml:"decode"`
		Format  string `toml:"format"`
		Timeout string `toml:"timeout"`

		Setting Map `toml:"setting"`
	}
)

func (site *httpSite) Route(name string, args ...Map) string {
	realName := fmt.Sprintf("%s.%s", site.name, name)
	return ark.Http.url.Route(realName, args...)
}

// Register 注册中心
func (site *httpSite) Register(name string, value Any, overrides ...bool) {
	key := fmt.Sprintf("%s.%s", site.name, name)

	switch val := value.(type) {
	case Router:
		if site.root != "" {
			if val.Uri != "" {
				val.Uri = site.root + val.Uri
			}
			if val.Uris != nil {
				for i, uri := range val.Uris {
					val.Uris[i] = site.root + uri
				}
			}
		}
		ark.Http.Router(key, val, overrides...)
	case Filter:
		ark.Http.Filter(key, val, overrides...)
	case RequestFilter:
		ark.Http.RequestFilter(key, val, overrides...)
	case ExecuteFilter:
		ark.Http.ExecuteFilter(key, val, overrides...)
	case ResponseFilter:
		ark.Http.ResponseFilter(key, val, overrides...)

	case Handler:
		ark.Http.Handler(key, val, overrides...)
	case FoundHandler:
		ark.Http.FoundHandler(key, val, overrides...)
	case ErrorHandler:
		ark.Http.ErrorHandler(key, val, overrides...)
	case FailedHandler:
		ark.Http.FailedHandler(key, val, overrides...)
	case DeniedHandler:
		ark.Http.DeniedHandler(key, val, overrides...)
	}

}
