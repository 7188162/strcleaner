package csvproc

import (
	"encoding/csv"
	"io"
	"os"
	"strings"

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

	var reader io.Reader = in
	if strings.EqualFold(conf.CodePage, "cp932") {
		reader = transform.NewReader(in, japanese.ShiftJIS.NewDecoder())
	}
	r := csv.NewReader(reader)
	r.LazyQuotes = true

	var out io.Writer = os.Stdout

	if strings.EqualFold(conf.CodePage, "utf8") && conf.Output.UTF8BOM {
		if _, err := out.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return err
		}
	}

	var outCloser io.Closer
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			return err
		}
		out = f
		outCloser = f
		defer outCloser.Close()
	}

	// ★ CP932 の場合は transform.Writer を Close して確実にフラッシュ
	if strings.EqualFold(conf.CodePage, "cp932") {
		tw := transform.NewWriter(out, japanese.ShiftJIS.NewEncoder())
		out = tw
		defer tw.Close()
	}

	w := csv.NewWriter(out)

	w.UseCRLF = strings.EqualFold(conf.Output.LineEnding, "crlf")

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
		RemoveChars:        "",                              
		RemoveCharsList:    conf.Normalize.RemoveChars.Items,
		RemoveCRLFOnly:     conf.Normalize.RemoveCRLFOnly,
		RemovePunctuation:  conf.Normalize.RemovePunctuation,
		RemoveSymbols:      conf.Normalize.RemoveSymbols,
		RemoveEmoji:        conf.Normalize.RemoveEmoji,
	}

	// 1→0 変換
	toZero := func(cols []int) []int {
		z := make([]int, 0, len(cols))
		for _, c := range cols {
			if c > 0 {
				z = append(z, c-1)
			}
		}
		return z
	}
	targetCols := toZero(conf.Columns)

	dedupeCols := targetCols
	if len(conf.Dedupe.Columns) > 0 {
		dedupeCols = toZero(conf.Dedupe.Columns)
	}
	sep := conf.Dedupe.Delimiter
	if sep == "" {
		sep = "|"
	}

	// --- デバッグ用カウンタ（-v 時に出力） ---
	var total, wrote, dropped, emptyKey int

	// --- ストリーミング書き出し路線か？ ---
	streamMode := !conf.Dedupe.Enabled || !conf.Dedupe.DropDuplicates || len(dedupeCols) == 0

	// ヘッダ処理
	var header []string
	if conf.HasHeader {
		rec, err := r.Read()
		if err == io.EOF {
			// 空CSV
			w.Flush()
			return w.Error()
		}
		if err != nil {
			return err
		}
		header = append([]string{}, rec...)
		if conf.Dedupe.Enabled && conf.Dedupe.AppendKey {
			header = append(header, conf.Dedupe.OutputHeader)
		}
		if err := w.Write(header); err != nil {
			return err
		}
		wrote++
	}

	if streamMode {
		// ====== ストリーミング書き出し（ここなら「ヘッダだけ」には絶対ならない） ======
		for {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf("CSV 読取エラー: %v", err)
				continue
			}
			total++

			// 正規化（行内キャッシュ）
			normalized := make(map[int]string)
			for _, col := range targetCols {
				if col >= 0 && col < len(rec) {
					cleaned := normalize.Clean(rec[col], opts)
					normalized[col] = cleaned
					if conf.Normalize.WriteBack {
						rec[col] = cleaned
					}
				}
			}

			// キー生成（append/replace用）
			if conf.Dedupe.Enabled && len(dedupeCols) > 0 {
				values := make([]string, 0, len(dedupeCols))
				for _, col := range dedupeCols {
					v := ""
					if conf.Dedupe.UseNormalized {
						if n, ok := normalized[col]; ok {
							v = n
						} else if col >= 0 && col < len(rec) {
							v = normalize.Clean(rec[col], opts)
						}
					} else if col >= 0 && col < len(rec) {
						v = rec[col]
					}
					values = append(values, v)
				}
				key := strings.Join(values, sep)
				if strings.TrimSpace(key) == "" {
					emptyKey++
				}
				if conf.Dedupe.ReplaceTarget && len(dedupeCols) > 0 {
					firstCol := dedupeCols[0]
					if firstCol >= 0 && firstCol < len(rec) {
						rec[firstCol] = key
					}
				}
				if conf.Dedupe.AppendKey {
					rec = append(rec, key)
				}
			}

			if err := w.Write(rec); err != nil {
				return err
			}
			wrote++
		}

		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
		log.Debugf("stream: rows_read=%d wrote=%d empty_keys=%d", total, wrote, emptyKey)
		return nil
	}

	// ====== ここからは drop_duplicates: true かつ dedupe 有効時（keep=first/last） ======
	type row struct {
		fields    []string
		key       string
		skipDedup bool // 空キーなど
	}
	var rows []row

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("CSV 読取エラー: %v", err)
			continue
		}
		total++

		normalized := make(map[int]string)
		for _, col := range targetCols {
			if col >= 0 && col < len(rec) {
				cleaned := normalize.Clean(rec[col], opts)
				normalized[col] = cleaned
				if conf.Normalize.WriteBack {
					rec[col] = cleaned
				}
			}
		}

		values := make([]string, 0, len(dedupeCols))
		for _, col := range dedupeCols {
			v := ""
			if conf.Dedupe.UseNormalized {
				if n, ok := normalized[col]; ok {
					v = n
				} else if col >= 0 && col < len(rec) {
					v = normalize.Clean(rec[col], opts)
				}
			} else if col >= 0 && col < len(rec) {
				v = rec[col]
			}
			values = append(values, v)
		}
		key := strings.Join(values, sep)

		skip := false
		if strings.TrimSpace(key) == "" {
			skip = true // 空キーはdrop対象外で必ず出力
			emptyKey++
		}

		if conf.Dedupe.ReplaceTarget && len(dedupeCols) > 0 && !skip {
			firstCol := dedupeCols[0]
			if firstCol >= 0 && firstCol < len(rec) {
				rec[firstCol] = key
			}
		}
		if conf.Dedupe.AppendKey {
			rec = append(rec, key)
		}

		rows = append(rows, row{fields: rec, key: key, skipDedup: skip})
	}

	seen := map[string]struct{}{}
	switch conf.Dedupe.Keep {
	case "last":
		keep := make([]bool, len(rows))
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i].skipDedup {
				keep[i] = true
				continue
			}
			k := rows[i].key
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				keep[i] = true
			} else {
				dropped++
			}
		}
		for i := 0; i < len(rows); i++ {
			if keep[i] {
				_ = w.Write(rows[i].fields)
				wrote++
			}
		}
	default: // first
		for i := 0; i < len(rows); i++ {
			if rows[i].skipDedup {
				_ = w.Write(rows[i].fields)
				wrote++
				continue
			}
			k := rows[i].key
			if _, ok := seen[k]; ok {
				dropped++
				continue
			}
			seen[k] = struct{}{}
			_ = w.Write(rows[i].fields)
			wrote++
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	log.Debugf("dedupe: rows_read=%d wrote=%d dropped=%d empty_keys=%d keep=%s",
		total, wrote, dropped, emptyKey, conf.Dedupe.Keep)
	return nil
}
