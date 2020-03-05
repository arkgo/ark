package ark

import (
	. "github.com/arkgo/base"
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

		Crypto   string `toml:"crypto"`
		Validate string `toml:"validate"`
		Format   string `toml:"format"`

		Setting Map `toml:"setting"`
	}
)
