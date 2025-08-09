package config

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type NormalizeConfig struct {
	ToUpper            bool   `mapstructure:"to_upper"`
	ToLower            bool   `mapstructure:"to_lower"`
	HalfKanaToFull     bool   `mapstructure:"half_kana_to_full"`
	FullKanaToHalf     bool   `mapstructure:"full_kana_to_half"`
	FullDigitToHalf    bool   `mapstructure:"full_digit_to_half"`
	ParenNumToHalf     bool   `mapstructure:"paren_num_to_half"`
	DashToHyphen       bool   `mapstructure:"dash_to_hyphen"`
	RemoveParens       bool   `mapstructure:"remove_parens"`
	RemoveNonPrintable bool   `mapstructure:"remove_non_printable"`
	RemoveHTML         bool   `mapstructure:"remove_html"`
	RemoveChars        string `mapstructure:"remove_chars"`
}

type Config struct {
	Columns   []int           `mapstructure:"columns"`
	CodePage  string          `mapstructure:"code_page"` // cp932|utf8
	Log       LogConfig       `mapstructure:"log"`
	Normalize NormalizeConfig `mapstructure:"normalize"`
	Timeout   time.Duration   `mapstructure:"timeout"`
}

func defaultConfig() Config {
	return Config{
		Columns:  []int{0},
		CodePage: "utf8",
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		Normalize: NormalizeConfig{
			FullDigitToHalf: true,
			DashToHyphen:    true,
		},
		Timeout: 10 * time.Minute,
	}
}

// Load merges: defaults < file < env < flags
func Load(cfgFile string, flags *pflag.FlagSet) (Config, error) {
    // 1 まずデフォルト値入りの構造体を作る
    c := defaultConfig()

    v := viper.New()
    if cfgFile != "" {
        v.SetConfigFile(cfgFile)
    }
    v.SetConfigType("yaml")
    v.SetEnvPrefix("STRCLEANER")
    v.AutomaticEnv()

    // 2 設定ファイル・環境変数・フラグで「上書き」する
    _ = v.ReadInConfig()
    _ = v.BindPFlags(flags)

    if err := v.Unmarshal(&c); err != nil { // ← デフォルト値を保持しつつマージ
        return Config{}, err
    }

    // 3 Level が空なら "info"
    if c.Log.Level == "" {
        c.Log.Level = "info"
    }
    if _, err := logrus.ParseLevel(c.Log.Level); err != nil {
        return Config{}, err
    }
    return c, nil
}
