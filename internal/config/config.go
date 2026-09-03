// Package config resolves ditty configuration from defaults, an optional
// config file, the environment, and command-line flags, and remembers
// which of those a value actually came from so `ditty config show
// --resolved` can report it (SPEC.md §8).
//
// There are no persistent flags on the root command (CLAUDE.md), so a
// Resolver is built per subcommand: each command constructs its own
// *pflag.FlagSet and hands it to Load, rather than one Resolver being
// shared globally.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is the environment-variable prefix for every ditty setting
// (DITTY_PORT, DITTY_BASE_PATH, ...). Dots in a key become underscores.
const EnvPrefix = "DITTY"

// ConfigFileName is the base name (without extension) ditty looks for in
// its config search path.
const ConfigFileName = "ditty"

// Origin identifies which layer of the precedence chain a value came from.
type Origin string

const (
	OriginDefault Origin = "default"
	OriginFile    Origin = "config file"
	OriginEnv     Origin = "env"
	OriginFlag    Origin = "flag"
)

// Resolver binds one command's flag set to a Viper instance and tracks,
// per key, which layer supplied the effective value. Precedence (lowest
// to highest): built-in defaults -> config file -> environment -> flags.
type Resolver struct {
	v          *viper.Viper
	fs         *pflag.FlagSet
	configFile string // path actually loaded, empty if none was found
}

// New creates a Resolver, searching the standard locations for a config
// file: ./ditty.yaml, $XDG_CONFIG_HOME/ditty/ditty.yaml, /etc/ditty/ditty.yaml.
// A missing config file is not an error — ditty runs with defaults/env/flags
// alone.
func New() (*Resolver, error) {
	v := viper.New()
	v.SetConfigName(ConfigFileName)
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		v.AddConfigPath(filepath.Join(xdg, "ditty"))
	} else if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "ditty"))
	}
	v.AddConfigPath("/etc/ditty")

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	r := &Resolver{v: v}
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	} else {
		r.configFile = v.ConfigFileUsed()
	}
	return r, nil
}

// BindFlagSet registers every flag in fs as a Viper key, with the flag's
// own default as the built-in default. It must be called once per command
// invocation, against that command's own local flag set.
func (r *Resolver) BindFlagSet(fs *pflag.FlagSet) error {
	r.fs = fs
	return r.v.BindPFlags(fs)
}

// ConfigFileUsed returns the path of the config file that was loaded, or
// "" if none was found.
func (r *Resolver) ConfigFileUsed() string {
	return r.configFile
}

// String returns the resolved string value for key.
func (r *Resolver) String(key string) string { return r.v.GetString(key) }

// Int returns the resolved int value for key.
func (r *Resolver) Int(key string) int { return r.v.GetInt(key) }

// Bool returns the resolved bool value for key.
func (r *Resolver) Bool(key string) bool { return r.v.GetBool(key) }

// Origin reports which layer supplied key's effective value.
func (r *Resolver) Origin(key string) Origin {
	if r.fs != nil {
		if f := r.fs.Lookup(key); f != nil && f.Changed {
			return OriginFlag
		}
	}
	envKey := EnvPrefix + "_" + strings.NewReplacer(".", "_", "-", "_").Replace(strings.ToUpper(key))
	if _, ok := os.LookupEnv(envKey); ok {
		return OriginEnv
	}
	if r.configFile != "" && r.v.InConfig(key) {
		return OriginFile
	}
	return OriginDefault
}

// Keys returns every bound key in a stable order, for `config show`.
func (r *Resolver) Keys() []string {
	if r.fs == nil {
		return nil
	}
	keys := make([]string, 0)
	r.fs.VisitAll(func(f *pflag.Flag) {
		keys = append(keys, f.Name)
	})
	return keys
}
