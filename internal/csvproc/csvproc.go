package csvproc

import (
	"encoding/csv"
	"io"
	"os"

	"github.com/yourorg/strcleaner/internal/config"
	"github.com/yourorg/strcleaner/internal/logging"
	"github.com/yourorg/strcleaner/internal/normalize"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func Process(inFile, outFile string, conf config.Config, log logging.Logger) error {
	in, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer in.Close()

        // 変換テーブルを先に用意（1→0 オフセット）
        zeroCols := make([]int, 0, len(conf.Columns))
        for _, c := range conf.Columns {
            if c <= 0 {
                continue                    // 0 以下は無視
            }
            zeroCols = append(zeroCols, c-1) // 1オリジン→0オリジン
        }

	var reader io.Reader = in
	if conf.CodePage == "cp932" {
		reader = transform.NewReader(in, japanese.ShiftJIS.NewDecoder())
	}
	r := csv.NewReader(reader)
	r.LazyQuotes = true

	var out io.Writer = os.Stdout
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	if conf.CodePage == "cp932" {
		out = transform.NewWriter(out, japanese.ShiftJIS.NewEncoder())
	}
	w := csv.NewWriter(out)

	opts := normalize.Options{
		ToUpper:            conf.Normalize.ToUpper,
		ToLower:            conf.Normalize.ToLower,
		HalfKanaToFull:     conf.Normalize.HalfKanaToFull,
		FullKanaToHalf:     conf.Normalize.FullKanaToHalf,
		FullDigitToHalf:    conf.Normalize.FullDigitToHalf,
		ParenNumToHalf:     conf.Normalize.ParenNumToHalf,
		DashToHyphen:       conf.Normalize.DashToHyphen,
		RemoveParens:       conf.Normalize.RemoveParens,
		RemoveNonPrintable: conf.Normalize.RemoveNonPrintable,
		RemoveHTML:         conf.Normalize.RemoveHTML,
		RemoveChars:        conf.Normalize.RemoveChars,
	}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("CSV 読取エラー: %v", err)
			continue
		}
//		for _, col := range conf.Columns {
                for _, col := range zeroCols {
			if col < len(rec) {
				rec[col] = normalize.Clean(rec[col], opts)
			}
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
