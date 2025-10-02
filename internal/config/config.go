package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// 文字列 or 配列を受け付けるための型
type MultiChars struct {
	Items []string
}

type NormalizeConfig struct {
	ToUpper            bool       `mapstructure:"to_upper"             yaml:"to_upper"`
	ToLower            bool       `mapstructure:"to_lower"             yaml:"to_lower"`
	HalfKanaToFull     bool       `mapstructure:"half_kana_to_full"    yaml:"half_kana_to_full"`
	FullKanaToHalf     bool       `mapstructure:"full_kana_to_half"    yaml:"full_kana_to_half"`
	FullDigitToHalf    bool       `mapstructure:"full_digit_to_half"   yaml:"full_digit_to_half"`
	ParenNumToHalf     bool       `mapstructure:"paren_num_to_half"    yaml:"paren_num_to_half"`
	DashToHyphen       bool       `mapstructure:"dash_to_hyphen"       yaml:"dash_to_hyphen"`
	RemoveParens       bool       `mapstructure:"remove_parens"        yaml:"remove_parens"`
	RemoveNonPrintable bool       `mapstructure:"remove_non_printable" yaml:"remove_non_printable"`
	RemoveCRLFOnly     bool       `mapstructure:"remove_crlf_only"     yaml:"remove_crlf_only"`
	RemoveHTML         bool       `mapstructure:"remove_html"          yaml:"remove_html"`
	RemoveChars        MultiChars `mapstructure:"remove_chars"         yaml:"remove_chars"`

	RemoveHTMLTags   []string `mapstructure:"remove_html_tags"     yaml:"remove_html_tags"`
	RemoveSubstrings []string `mapstructure:"remove_substrings"    yaml:"remove_substrings"`

	// TrimSpaces        bool `mapstructure:"trim_spaces"          yaml:"trim_spaces"`
	// TrimHyphens       bool `mapstructure:"trim_hyphens"         yaml:"trim_hyphens"`
	// TrimUnderscores   bool `mapstructure:"trim_underscores"     yaml:"trim_underscores"`
	// TrimChars         MultiChars `mapstructure:"trim_chars"           yaml:"trim_chars"`

	WriteBack bool `mapstructure:"write_back"           yaml:"write_back"`

	RemovePunctuation bool `mapstructure:"remove_punctuation"   yaml:"remove_punctuation"`
	RemoveSymbols     bool `mapstructure:"remove_symbols"       yaml:"remove_symbols"`
	RemoveEmoji       bool `mapstructure:"remove_emoji"         yaml:"remove_emoji"`
}

type DedupeConfig struct {
	Enabled        bool   `mapstructure:"enabled"`          // 重複排除を有効化
	Columns        []int  `mapstructure:"columns"`          // キー作成に使う列(1オリジン)。未指定なら Config.Columns
	AppendKey      bool   `mapstructure:"append_key"`       // キー列を末尾に追加
	ReplaceTarget  bool   `mapstructure:"replace_target"`   // columns先頭列をキーで置換
	DropDuplicates bool   `mapstructure:"drop_duplicates"`  // 既出キーの行を出力しない
	Keep           string `mapstructure:"keep"`             // first|last（DropDuplicates時の残し方）
	OutputHeader   string `mapstructure:"output_header"`    // AppendKey時のヘッダ名
	Delimiter      string `mapstructure:"delimiter"`        // 連結区切り
	UseNormalized  bool   `mapstructure:"use_normalized"`   // キー生成に正規化後を使うか(既定true)
	IgnoreEmptyKey bool   `mapstructure:"ignore_empty_key"` // ★追加：空キーはdrop対象外
}

type OutputConfig struct {
	LineEnding string `mapstructure:"line_ending"` // "crlf" | "lf" (default: "crlf")
	UTF8BOM    bool   `mapstructure:"utf8_bom"`    // default: true（UTF-8 のときのみ有効）
}

type Config struct {
	Columns   []int           `mapstructure:"columns"`    // 正規化対象列(1オリジン)
	CodePage  string          `mapstructure:"code_page"`  // cp932|utf8
	HasHeader bool            `mapstructure:"has_header"` // 先頭行はヘッダ行か
	Log       LogConfig       `mapstructure:"log"`
	Normalize NormalizeConfig `mapstructure:"normalize"`
	Dedupe    DedupeConfig    `mapstructure:"dedupe"`
	Output    OutputConfig    `mapstructure:"output"`
	Timeout   time.Duration   `mapstructure:"timeout"`
}

func (m *MultiChars) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		var s string
		if err := n.Decode(&s); err != nil {
			return err
		}
		m.Items = []string{s}
		return nil
	case yaml.SequenceNode:
		var a []string
		if err := n.Decode(&a); err != nil {
			return err
		}
		m.Items = a
		return nil
	default:
		return fmt.Errorf("remove_chars must be string or array of strings")
	}
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
			WriteBack:       true,
		},
		Dedupe: DedupeConfig{
			Enabled:        false,
			AppendKey:      false,
			ReplaceTarget:  false,
			DropDuplicates: false,
			Keep:           "first",
			Delimiter:      "|",
			OutputHeader:   "__dedupe_key",
			UseNormalized:  true,
			IgnoreEmptyKey: true, // ★既定で“空キーは落とさない”
		},
		Output: OutputConfig{
			LineEnding: "crlf",
			UTF8BOM:    true,
		},
		Timeout: 10 * time.Minute,
	}
}

// Load merges defaults < file < env < flags
// Load merges defaults < file < env < flags
func Load(cfgFile string, flags *pflag.FlagSet, noStrict bool) (Config, error) {
	c := defaultConfig()

	v := viper.New()
	v.SetEnvPrefix("STRCLEANER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	// 1) ファイル読込
	if cfgFile != "" {
		if noStrict {
			// 非厳格：Viperに読ませる（未知キーを無視）
			v.SetConfigFile(cfgFile)
			if err := v.ReadInConfig(); err != nil {
				return Config{}, err
			}
			// MultiChars 対応の DecodeHook を使って Unmarshal
			if err := v.Unmarshal(&c, func(dc *mapstructure.DecoderConfig) {
				dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(multiCharsDecodeHook())
				dc.TagName = "mapstructure"
			}); err != nil {
				return Config{}, err
			}
		} else {
			// 厳格：yaml.v3 で KnownFields(true) により未知キー検出
			validated, err := strictYAMLValidate(cfgFile, c)
			if err != nil {
				return Config{}, err
			}
			c = validated
		}
	}

	// 2) ENV で上書き（ファイルをViperに読ませていない場合も、ENVだけ適用）
	if err := v.Unmarshal(&c, func(dc *mapstructure.DecoderConfig) {
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(multiCharsDecodeHook())
		dc.TagName = "mapstructure"
	}); err != nil {
		return Config{}, err
	}

	// 3) CLI フラグで上書き（指定があった項目のみ）
	if flags != nil {
		if f := flags.Lookup("output.line_ending"); f != nil && f.Changed {
			c.Output.LineEnding = strings.ToLower(f.Value.String())
		}
		if f := flags.Lookup("output.utf8_bom"); f != nil && f.Changed {
			if b, err := strconv.ParseBool(f.Value.String()); err == nil {
				c.Output.UTF8BOM = b
			}
		}
		// 将来的に他の項目もCLIで上書きしたければここに追記
	}

	// 4) バリデーション/正規化
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if _, err := logrus.ParseLevel(c.Log.Level); err != nil {
		return Config{}, err
	}
	switch strings.ToLower(c.Output.LineEnding) {
	case "", "crlf":
		c.Output.LineEnding = "crlf"
	case "lf":
	default:
		c.Output.LineEnding = "crlf"
	}
	if !strings.EqualFold(c.CodePage, "utf8") && !strings.EqualFold(c.CodePage, "cp932") {
		return Config{}, fmt.Errorf("unsupported code_page: %s (use utf8 or cp932)", c.CodePage)
	}
	for _, x := range c.Columns {
		if x <= 0 {
			return Config{}, fmt.Errorf("columns must be 1-origin positive integers: %v", c.Columns)
		}
	}
	if len(c.Dedupe.Columns) > 0 {
		for _, x := range c.Dedupe.Columns {
			if x <= 0 {
				return Config{}, fmt.Errorf("dedupe.columns must be 1-origin positive integers: %v", c.Dedupe.Columns)
			}
		}
	}
	// Keep 正規化
	switch c.Dedupe.Keep {
	case "", "first":
		c.Dedupe.Keep = "first"
	case "last":
	default:
		c.Dedupe.Keep = "first"
	}

	return c, nil
}

func multiCharsDecodeHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data interface{}) (interface{}, error) {
		// from string → MultiChars
		if to == reflect.TypeOf(MultiChars{}) && from.Kind() == reflect.String {
			return MultiChars{Items: []string{data.(string)}}, nil
		}
		// from []interface{} → MultiChars（YAMLの配列）
		if to == reflect.TypeOf(MultiChars{}) && from.Kind() == reflect.Slice {
			raw := data.([]interface{})
			items := make([]string, 0, len(raw))
			for _, v := range raw {
				items = append(items, fmt.Sprint(v))
			}
			return MultiChars{Items: items}, nil
		}
		return data, nil
	}
}

func strictYAMLValidate(path string, base Config) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := base
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // ★ 未知キー・階層ズレを検出
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config file (unknown/misplaced key?): %w", err)
	}
	return cfg, nil
}
