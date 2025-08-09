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

type DedupeConfig struct {
	Enabled        bool   `mapstructure:"enabled"`         // 重複排除を有効化
	Columns        []int  `mapstructure:"columns"`         // キー作成に使う列(1オリジン)。未指定なら Config.Columns
	AppendKey      bool   `mapstructure:"append_key"`      // キー列を末尾に追加
	ReplaceTarget  bool   `mapstructure:"replace_target"`  // columns先頭列をキーで置換
	DropDuplicates bool   `mapstructure:"drop_duplicates"` // 既出キーの行を出力しない
	Keep           string `mapstructure:"keep"`            // first|last（DropDuplicates時の残し方）
	OutputHeader   string `mapstructure:"output_header"`   // AppendKey時のヘッダ名
	Delimiter      string `mapstructure:"delimiter"`       // 連結区切り
}

type Config struct {
	Columns   []int           `mapstructure:"columns"`    // 正規化対象列(1オリジン)
	CodePage  string          `mapstructure:"code_page"`  // cp932|utf8
	HasHeader bool            `mapstructure:"has_header"` // 先頭行はヘッダ行か
	Log       LogConfig       `mapstructure:"log"`
	Normalize NormalizeConfig `mapstructure:"normalize"`
	Dedupe    DedupeConfig    `mapstructure:"dedupe"`
	Timeout   time.Duration   `mapstructure:"timeout"`
}

func defaultConfig() Config {
	return Config{
		Columns:   []int{1}, // 1オリジン
		CodePage:  "utf8",
		HasHeader: false,
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		Normalize: NormalizeConfig{
			FullDigitToHalf: true,
			DashToHyphen:    true,
		},
		Dedupe: DedupeConfig{
			Enabled:        false,
			AppendKey:      false,
			ReplaceTarget:  false,
			DropDuplicates: false,
			Keep:           "first",
			Delimiter:      "|",
			OutputHeader:   "__dedupe_key",
		},
		Timeout: 10 * time.Minute,
	}
}

// Load merges defaults < file < env < flags
func Load(cfgFile string, flags *pflag.FlagSet) (Config, error) {
	c := defaultConfig()

	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}
	v.SetConfigType("yaml")
	v.SetEnvPrefix("STRCLEANER")
	v.AutomaticEnv()

	_ = v.ReadInConfig()
	_ = v.BindPFlags(flags)

	if err := v.Unmarshal(&c); err != nil {
		return Config{}, err
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if _, err := logrus.ParseLevel(c.Log.Level); err != nil {
		return Config{}, err
	}
	// Keep 値の正規化
	switch c.Dedupe.Keep {
	case "", "first":
		c.Dedupe.Keep = "first"
	case "last":
	default:
		c.Dedupe.Keep = "first"
	}
	return c, nil
}
