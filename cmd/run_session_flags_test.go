package cmd

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestRegisterSessionFlags_DefaultsMatchSpec(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerSessionFlags(fs)

	tests := []struct {
		name  string
		short string
		want  string
	}{
		{"scrollback-bytes", "S", "262144"},
		{"chunk-bytes", "u", "32768"},
		{"flush-interval", "m", (5 * time.Millisecond).String()},
		{"max-clients", "M", "0"},
		{"writable", "w", "false"},
	}
	for _, tt := range tests {
		f := fs.Lookup(tt.name)
		if f == nil {
			t.Fatalf("flag %q not registered", tt.name)
		}
		if f.Shorthand != tt.short {
			t.Errorf("flag %q shorthand = %q, want %q", tt.name, f.Shorthand, tt.short)
		}
		if f.DefValue != tt.want {
			t.Errorf("flag %q default = %q, want %q", tt.name, f.DefValue, tt.want)
		}
	}
}

func TestRunCmd_SessionFlagsRegistered(t *testing.T) {
	for _, name := range []string{"scrollback-bytes", "chunk-bytes", "flush-interval", "max-clients", "writable"} {
		if runCmd.Flags().Lookup(name) == nil {
			t.Errorf("ditty run is missing --%s", name)
		}
	}
}
