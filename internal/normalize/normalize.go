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
	RemoveHTML         bool
	RemoveChars        string
}

var (
	reParenNum      = regexp.MustCompile(`［?([０-９])］?|\(([０-９])\)|【([０-９])】|〔([０-９])〕|[①-⑨]`)
	reDash          = regexp.MustCompile(`[ー－―–—‐]`)
	reParens        = regexp.MustCompile(`[()\[\]{}「」『』【】［］〔〕（）]`)
	reNonPrintable  = regexp.MustCompile(`[\p{Cc}\p{Cf}]`)
	reHTMLTag       = regexp.MustCompile(`</?[^>]+?>`)
)

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

	// 5. HTML
	if opt.RemoveHTML {
		s = reHTMLTag.ReplaceAllString(s, "")
	}

	// 6. 非印刷
	if opt.RemoveNonPrintable {
		s = reNonPrintable.ReplaceAllString(s, "")
	}

	// 7. 個別削除
	if opt.RemoveChars != "" {
		s = removeChars(s, opt.RemoveChars)
	}

	return strings.TrimSpace(s)
}

// func transformString(t width.Transformer, s string) string {
// 	return strings.Map(func(r rune) rune {
// 		r, _ = t.TransformRune(r)
// 		return r
// 	}, s)
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
