package normalize

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

type Options struct {
	ToUpper            bool
	ToLower            bool
	HalfKanaToFull     bool
	FullKanaToHalf     bool
	FullDigitToHalf    bool
	ParenNumToHalf     bool
	DashToHyphen       bool
	RemoveParens       bool
	RemoveNonPrintable bool
	RemoveCRLFOnly     bool
	RemoveHTML         bool
	RemoveChars        string
	RemoveCharsList    []string

	RemoveHTMLTags   []string       // 指定タグのみ除去
	removeTagsRe     *regexp.Regexp // 事前コンパイル済み
	RemoveSubstrings []string       // 指定の“部分文字列”をリテラル除去

	RemovePunctuation bool
	RemoveSymbols     bool
	RemoveEmoji       bool
}

var (
	reParenNum     = regexp.MustCompile(`［?([０-９])］?|\(([０-９])\)|【([０-９])】|〔([０-９])〕|[①-⑨]`)
	reDash         = regexp.MustCompile(`[ー－―–—‐]`)
	reParens       = regexp.MustCompile(`[()\[\]{}「」『』【】［］〔〕（）]`)
	reNonPrintable = regexp.MustCompile(`[\p{Cc}\p{Cf}]`)
	reHTMLTag      = regexp.MustCompile(`</?[^>]+?>`)
)

func compileTagRegex(tags []string) *regexp.Regexp {
	if len(tags) == 0 {
		return nil
	}
	esc := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		esc = append(esc, regexp.QuoteMeta(t))
	}
	if len(esc) == 0 {
		return nil
	}
	// (?is) … i=case-insensitive, s=dotall（改行にもマッチ）
	// </? TAG \b [^>]* >
	pat := `(?is)</?\s*(?:` + strings.Join(esc, "|") + `)\b[^>]*>`
	return regexp.MustCompile(pat)
}

func (o *Options) Prepare() {
	if len(o.RemoveHTMLTags) > 0 {
		o.removeTagsRe = compileTagRegex(o.RemoveHTMLTags)
	} else {
		o.removeTagsRe = nil
	}
}

func Clean(s string, opt Options) string {
	// 0. NFKC
	s = norm.NFKC.String(s)

	// 1. 幅変換
	if opt.HalfKanaToFull {
		s = transformString(width.Widen, s)
	}
	if opt.FullKanaToHalf {
		s = transformString(width.Narrow, s)
	}
	if opt.FullDigitToHalf {
		s = transformString(width.Narrow, s)
	}
	if opt.ParenNumToHalf {
		s = reParenNum.ReplaceAllStringFunc(s, fullNumToHalf)
	}

	// 2. ハイフン
	if opt.DashToHyphen {
		s = reDash.ReplaceAllString(s, "-")
	}

	// 3. ケース
	if opt.ToUpper {
		s = strings.ToUpper(s)
	} else if opt.ToLower {
		s = strings.ToLower(s)
	}

	// 4. カッコ
	if opt.RemoveParens {
		s = reParens.ReplaceAllString(s, "")
	}

	// 5-1. 特定タグだけ除去（中身は保持）
	if opt.removeTagsRe != nil {
		s = opt.removeTagsRe.ReplaceAllString(s, "")
	}

	// 5-2. HTML 全除去（remove_html: true のとき）
	if opt.RemoveHTML {
		s = reHTMLTag.ReplaceAllString(s, "")
	}

	// 6. 非印刷
	if opt.RemoveNonPrintable {
		s = reNonPrintable.ReplaceAllString(s, "")
	}

	// ★ カテゴリ系の削除
	if opt.RemovePunctuation {
		s = removeByPredicate(s, func(r rune) bool { return unicode.In(r, unicode.Punct) })
	}
	if opt.RemoveSymbols {
		s = removeByPredicate(s, func(r rune) bool { return unicode.In(r, unicode.Symbol) })
	}
	if opt.RemoveEmoji {
		s = removeByPredicate(s, isEmoji)
	}

	// 6a. 改行だけ削除（CR/LF のみ）
	if opt.RemoveCRLFOnly {
		s = strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' {
				return -1
			}
			return r
		}, s)
	}

	// 6b. 非印刷（制御/書式）を一括削除
	if opt.RemoveNonPrintable {
		s = reNonPrintable.ReplaceAllString(s, "")
	}

	// 個別文字削除（配列も併用）
	if opt.RemoveChars != "" || len(opt.RemoveCharsList) > 0 {
		var b strings.Builder
		for _, s2 := range opt.RemoveCharsList {
			b.WriteString(s2)
		}
		b.WriteString(opt.RemoveChars)
		s = removeChars(s, b.String())
	}

	// 7. 個別削除
	if opt.RemoveChars != "" {
		s = removeChars(s, opt.RemoveChars)
	}

	// 8. 特定部分文字列のリテラル除去
	if len(opt.RemoveSubstrings) > 0 {
		for _, sub := range opt.RemoveSubstrings {
			if sub == "" {
				continue
			}
			s = strings.ReplaceAll(s, sub, "")
		}
	}

	// 9. 個別文字（集合）削除（配列＋文字列を合算）
	if opt.RemoveChars != "" || len(opt.RemoveCharsList) > 0 {
		var b strings.Builder
		for _, set := range opt.RemoveCharsList {
			b.WriteString(set)
		}
		b.WriteString(opt.RemoveChars)
		s = removeChars(s, b.String())
	}

	return strings.TrimSpace(s)
}

//	func transformString(t width.Transformer, s string) string {
//		return strings.Map(func(r rune) rune {
//			r, _ = t.TransformRune(r)
//			return r
//		}, s)
func transformString(t transform.Transformer, s string) string {
	out, _, _ := transform.String(t, s) // ★ ここを変更
	return out

}

func fullNumToHalf(m string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case IsZenkakuDigit(r):
			return r - '０' + '0'
		case r >= '①' && r <= '⑨':
			return r - '①' + '1'
		default:
			return r
		}
	}, m)
}

func removeChars(s, chars string) string {
	set := make(map[rune]struct{}, len(chars))
	for _, r := range chars {
		set[r] = struct{}{}
	}
	return strings.Map(func(r rune) rune {
		if _, ok := set[r]; ok {
			return -1
		}
		return r
	}, s)
}

func IsZenkakuDigit(r rune) bool {
	return unicode.In(r, unicode.Number) && r >= '０' && r <= '９'
}

func removeByPredicate(s string, pred func(rune) bool) string {
	return strings.Map(func(r rune) rune {
		if pred(r) {
			return -1
		}
		return r
	}, s)
}

func isEmoji(r rune) bool {
	switch {
	case r == 0x200D || r == 0xFE0F || r == 0x20E3: // ZWJ / VS-16 / keycap
		return true
	case r >= 0x1F3FB && r <= 0x1F3FF: // skin tones
		return true
	case r >= 0x1F600 && r <= 0x1F64F: // Emoticons
		return true
	case r >= 0x1F300 && r <= 0x1F5FF: // Misc Symbols and Pictographs
		return true
	case r >= 0x1F680 && r <= 0x1F6FF: // Transport & Map
		return true
	case r >= 0x2600 && r <= 0x26FF: // Misc Symbols
		return true
	case r >= 0x2700 && r <= 0x27BF: // Dingbats
		return true
	case r >= 0x1F900 && r <= 0x1F9FF: // Supplemental Symbols & Pictographs
		return true
	case r >= 0x1FA70 && r <= 0x1FAFF: // Symbols & Pictographs Extended-A
		return true
	case r >= 0x1F1E6 && r <= 0x1F1FF: // Regional Indicator
		return true
	default:
		return false
	}
}
