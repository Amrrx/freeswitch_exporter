package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var (
	// This timestamp format differs from RFC3339Nano by using .000 instead
	// of .999999999 which changes the timestamp from 9 variable to 3 fixed
	// decimals (.130 instead of .130987456).
	timestampFormat = log.TimestampFormat(
		func() time.Time { return time.Now().UTC() },
		"2006-01-02T15:04:05.000Z07:00",
	)
)

// AllowedLevel is a settable identifier for the minimum level a log entry
// must be have.
type AllowedLevel struct {
	s string
	o level.Option
}

func (l *AllowedLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	type plain string
	if err := unmarshal((*plain)(&s)); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	lo := &AllowedLevel{}
	if err := lo.Set(s); err != nil {
		return err
	}
	*l = *lo
	return nil
}

func (l *AllowedLevel) String() string {
	return l.s
}

// Set updates the value of the allowed level.
func (l *AllowedLevel) Set(s string) error {
	switch s {
	case "debug":
		l.o = level.AllowDebug()
	case "info":
		l.o = level.AllowInfo()
	case "warn":
		l.o = level.AllowWarn()
	case "error":
		l.o = level.AllowError()
	default:
		return fmt.Errorf("unrecognized log level %q", s)
	}
	l.s = s
	return nil
}

// AllowedFormat is a settable identifier for the output format that the logger can have.
type AllowedFormat struct {
	s string
}

func (f *AllowedFormat) String() string {
	return f.s
}

// Set updates the value of the allowed format.
func (f *AllowedFormat) Set(s string) error {
	switch s {
	case "logfmt", "json":
		f.s = s
	default:
		return fmt.Errorf("unrecognized log format %q", s)
	}
	return nil
}

// Config is a struct containing configurable settings for the logger
type Config struct {
	Level  *AllowedLevel
	Format *AllowedFormat
}

// New returns a new leveled oklog logger. Each logged line will be annotated
// with a timestamp. The output always goes to stderr.
// func New(config *Config) log.Logger {
// 	var l log.Logger
// 	if config.Format != nil && config.Format.s == "json" {
// 		l = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
// 	} else {
// 		l = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
// 	}

// 	if config.Level != nil {
// 		l = log.With(l, "timestamp", timestampFormat, "caller", log.Caller(5), "cnfc_uuid", cnfc_uuid, "cnf_uuid", cnf_uuid, "ns_uuid", ns_uuid)
// 		l = level.NewFilter(l, config.Level.o)
// 	} else {
// 		l = log.With(l, "timestamp", timestampFormat, "caller", log.DefaultCaller, "cnfc_uuid", cnfc_uuid, "cnf_uuid", cnf_uuid, "ns_uuid", ns_uuid)
// 	}
// 	return l
// }
func New(config *Config) log.Logger {
	if config == nil {
		// Handle nil config. Could return a default logger or panic with a clear error message.
		panic("config cannot be nil")
	}

	var l log.Logger
	if config.Format != nil && config.Format.s == "json" {
		l = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	} else {
		l = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	}

	// Initialize the logger with additional context
	baseLogger := log.With(l, "timestamp", timestampFormat, "caller", log.Caller(5), "cnfc_uuid", cnfc_uuid, "cnf_uuid", cnf_uuid, "ns_uuid", ns_uuid)

	if config.Level != nil && config.Level.o != nil {
		// Use the provided level option only if it's not nil
		l = level.NewFilter(baseLogger, config.Level.o)
	} else {
		// Use default caller if level option is not provided
		l = log.With(baseLogger, "caller", log.DefaultCaller)
	}

	return l
}

// NewDynamic returns a new leveled logger. Each logged line will be annotated
// with a timestamp. The output always goes to stderr. Some properties can be
// changed, like the level.
func NewDynamic(config *Config) *logger {
	var l log.Logger
	if config.Format != nil && config.Format.s == "json" {
		l = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	} else {
		l = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	}

	lo := &logger{
		base:    l,
		leveled: l,
	}

	if config.Level != nil {
		lo.SetLevel(config.Level)
	}

	return lo
}

type logger struct {
	base         log.Logger
	leveled      log.Logger
	currentLevel *AllowedLevel
	mtx          sync.Mutex
}

// Log implements logger.Log.
func (l *logger) Log(keyvals ...interface{}) error {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.leveled.Log(keyvals...)
}

// SetLevel changes the log level.
func (l *logger) SetLevel(lvl *AllowedLevel) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if lvl == nil {
		l.leveled = log.With(l.base, "timestamp", timestampFormat, "caller", log.DefaultCaller, "cnfc_uuid", cnfc_uuid, "cnf_uuid", cnf_uuid, "ns_uuid", ns_uuid)
		l.currentLevel = nil
		return
	}

	if l.currentLevel != nil && l.currentLevel.s != lvl.s {
		_ = l.base.Log("message", "Log level changed", "prev", l.currentLevel, "current", lvl, "cnfc_uuid", cnfc_uuid, "cnf_uuid", cnf_uuid, "ns_uuid", ns_uuid)
	}
	l.currentLevel = lvl
	l.leveled = level.NewFilter(log.With(l.base, "timestamp", timestampFormat, "caller", log.Caller(5), "cnfc_uuid", cnfc_uuid, "cnf_uuid", cnf_uuid, "ns_uuid", ns_uuid), lvl.o)
}
