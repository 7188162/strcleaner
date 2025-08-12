package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yourorg/strcleaner/internal/config"
	"github.com/yourorg/strcleaner/internal/csvproc"
	"github.com/yourorg/strcleaner/internal/logging"
)

var (
	exitOK      = 0
	exitErrConf = 1
	exitErrExec = 2
)

var (
	cfgPath   string
	inputCSV  string
	outputCSV string
	quiet     bool
	verbose   bool
	noStrict  bool
)

func init() {
	f := rootCmd.Flags()
	f.StringVarP(&cfgPath, "config", "c", "", "設定ファイル (yaml/toml)")
	f.StringVarP(&inputCSV, "input", "i", "", "入力 CSV ファイル (必須)")
	f.StringVarP(&outputCSV, "output", "o", "", "出力 CSV ファイル (省略時 STDOUT)")
	f.BoolVarP(&quiet, "quiet", "q", false, "quiet モード (結果のみ)")
	f.BoolVarP(&quiet, "silent", "s", false, "silent モード (結果のみ)")
	f.BoolVarP(&verbose, "verbose", "v", false, "verbose モード")

	f.String("output.line_ending", "", "出力改行コード (crlf|lf)")
	f.Bool("output.utf8_bom", false, "UTF-8 の BOM を付与する (true/false)")

	f.BoolVar(&noStrict, "no-strict-config", false, "設定ファイルの未知キーを許容する（厳格チェックを無効化）")

	_ = rootCmd.MarkFlagRequired("input")
}

var rootCmd = &cobra.Command{
	Use:   "strcleaner",
	Short: "CSV 文字列正規化ツール",
	RunE: func(cmd *cobra.Command, args []string) error {
		conf, err := config.Load(cfgPath, cmd.Flags(), noStrict)
		if err != nil {
			return err
		}
		log := logging.New(conf.Log)
		if quiet {
			log.SetLevel(logging.LevelQuiet)
		} else if verbose {
			log.SetLevel(logging.LevelVerbose)
		}
		return csvproc.Process(inputCSV, outputCSV, conf, log)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitErrExec)
	}
	os.Exit(exitOK)
}
