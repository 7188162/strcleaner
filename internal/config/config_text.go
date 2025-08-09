package config

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
)

func TestLoadDefault(t *testing.T) {
	c, err := Load("", pflag.NewFlagSet("test", pflag.ContinueOnError))
	if err != nil {
		t.Fatal(err)
	}
	if c.CodePage != "utf8" {
		t.Fatalf("default code page not utf8: %s", c.CodePage)
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("STRCLEANER_COLUMNS", "1,2")
	defer os.Unsetenv("STRCLEANER_COLUMNS")
	c, _ := Load("", pflag.NewFlagSet("test", pflag.ContinueOnError))
	if len(c.Columns) == 0 || c.Columns[0] != 1 {
		t.Fatalf("env override failed: %+v", c.Columns)
	}
}
