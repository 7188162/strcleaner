package normalize

import "testing"

func TestClean_Full(t *testing.T) {
	opts := Options{
		ToLower:            true,
		RemoveHTML:         true,
		RemoveParens:       true,
		RemoveNonPrintable: true,
		RemoveChars:        " -.," + "\u3000",
	}
	got := Clean("Ａ<sup>Ｂ</sup>（Ｃ）\n", opts)
	want := "abc"
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}
