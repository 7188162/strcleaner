package logging

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/yourorg/strcleaner/internal/config"
)

// 型エイリアス（他パッケージから *logrus.Logger を返したいだけ）
type Logger = *logrus.Logger

const (
	LevelQuiet   = logrus.ErrorLevel // --quiet / --silent
	LevelVerbose = logrus.DebugLevel // --verbose
)

// New returns a pre-configured *logrus.Logger.
func New(cfg config.LogConfig) Logger {
	log := logrus.New()

	// ---- ログレベル ----------------------------------------------------
	level := cfg.Level
	if level == "" {
		level = "info" // デフォルト
	}
	if lvl, err := logrus.ParseLevel(level); err == nil {
		log.SetLevel(lvl)
	} else {
		log.SetLevel(logrus.InfoLevel) // フォールバック
	}

	// ---- フォーマット ---------------------------------------------------
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// ---- 出力先 ---------------------------------------------------------
	var out io.Writer = os.Stdout
	switch cfg.Output {
	case "stderr":
		out = os.Stderr
	case "":
		// 何も指定がなければ stdout（既定）
	}
	log.SetOutput(out)

	return log
}
