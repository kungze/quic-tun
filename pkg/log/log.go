package log

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Field is an alias for the field structure in the zap log frame.
type Field = zapcore.Field

// Logger defines the capabilities that a logger has.
type Logger interface {
	// Output info log
	Info(msg string, fields ...Field)
	Infof(fromat string, v ...any)
	Infow(msg string, keysAndValues ...any)
	// Output debug log
	Debug(msg string, fields ...Field)
	Debugf(fromat string, v ...any)
	Debugw(msg string, keysAndValues ...any)
	// Output warning log
	Warn(msg string, fields ...Field)
	Warnf(format string, v ...any)
	Warnw(msg string, keysAndValues ...any)
	// Output error log
	Error(msg string, fields ...Field)
	Errorf(format string, v ...any)
	Errorw(msg string, keysAndValues ...any)
	// Output panic log
	Panic(msg string, fields ...Field)
	Panicf(format string, v ...any)
	Panicw(msg string, keysAndValues ...any)
	// Output fatal log
	Fatal(msg string, fields ...Field)
	Fatalf(format string, v ...any)
	Fatalw(msg string, keysAndValues ...any)

	// Fulsh calls the underlying ZAP Core's Sync method, flushing any buffered log
	// entries. Applications should take care to call Sync before exiting.
	Flush()
	// WithValues adds some key-value pairs of context to a logger.
	WithValues(keysAndValues ...any) Logger
	// WithName adds a new element to the logger's name.
	// Successive calls with WithName continue to append
	// suffixes to the logger's name.
	WithName(name string) Logger
	// WithContext returns a copy of context in which the log value is set.
	WithContext(ctx context.Context) context.Context
}

type logger struct {
	zlogger *zap.Logger
}

var (
	std = New(NewOptions()) // Define logs that can be used directly
	mu  sync.Mutex
)

// Init initializes logger with specified options.
func Init(opts *Options) {
	mu.Lock()
	defer mu.Unlock()
	std = New(opts)
}

// New create logger by opts which can custmoized by command arguments and config file.
func New(opts *Options) *logger {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	zc := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: opts.Development,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:          opts.Format,
		DisableCaller:     opts.DisableCaller,
		DisableStacktrace: opts.DisableStacktrace,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      opts.OutputPaths,
		ErrorOutputPaths: opts.ErrorOutputPaths,
	}
	l, err := zc.Build(zap.AddStacktrace(zapcore.PanicLevel), zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	return &logger{zlogger: l.Named(opts.Name)}
}

func (l *logger) Flush() {
	_ = l.zlogger.Sync()
}

func Flush() { std.Flush() }

// handleFields converts a bunch of arbitrary key-value pairs into Zap fields.
func handleFields(l *zap.Logger, args []any) []zap.Field {
	if len(args) == 0 {
		return nil
	}

	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args); {
		// check just in case for strongly-typed Zap fields, which is illegal (since
		// it breaks implementation agnosticism), so we can give a better error message.
		if _, ok := args[i].(zap.Field); ok {
			l.DPanic("strongly-typed Zap Field passed to logr", zap.Any("zap field", args[i]))

			break
		}

		// process a key-value pair, ensuring that the key is a string
		key, val := args[i], args[i+1]
		keyStr, isString := key.(string)
		if !isString {
			// if the key isn't a string, DPanic and stop logging
			l.DPanic(
				"non-string key argument passed to logging, ignoring all later arguments",
				zap.Any("invalid key", key),
			)

			break
		}

		fields = append(fields, zap.Any(keyStr, val))
		i += 2
	}

	return fields
}

func WithValues(keysAndValues ...any) Logger { return std.WithValues(keysAndValues...) }

func (l *logger) WithValues(keysAndValues ...any) Logger {
	newLogger := l.zlogger.With(handleFields(l.zlogger, keysAndValues)...)
	return NewLogger(newLogger)
}

func WithName(name string) Logger { return std.WithName(name) }

func (l *logger) WithName(name string) Logger {
	newLogger := l.zlogger.Named(name)
	return NewLogger(newLogger)
}

// NewLogger returns a new Logger.
func NewLogger(l *zap.Logger) Logger {
	return &logger{
		zlogger: l,
	}
}

func WithContext(ctx context.Context) context.Context {
	return std.WithContext(ctx)
}

// contextKey is how we find Loggers in a context.Context.
type contextKey struct{}

func (l *logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext returns the value of the log key on the ctx.
func FromContext(ctx context.Context) Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(contextKey{}).(Logger); ok {
			return logger
		}
	}

	return WithName("Unknown-Context")
}

func (l *logger) Info(msg string, fields ...Field) {
	l.zlogger.Info(msg, fields...)
}

func (l *logger) Infof(format string, v ...any) {
	l.zlogger.Sugar().Infof(format, v)
}

func (l *logger) Infow(msg string, keysAndValues ...any) {
	l.zlogger.Sugar().Infow(msg, keysAndValues...)
}

func (l *logger) Debug(msg string, fields ...Field) {
	l.zlogger.Debug(msg, fields...)
}

func (l *logger) Debugf(format string, v ...any) {
	l.zlogger.Sugar().Debugf(format, v)
}

func (l *logger) Debugw(msg string, keysAndValues ...any) {
	l.zlogger.Sugar().Debugw(msg, keysAndValues...)
}

func (l *logger) Warn(msg string, fields ...Field) {
	l.zlogger.Warn(msg, fields...)
}

func (l *logger) Warnf(format string, v ...any) {
	l.zlogger.Sugar().Warnf(format, v)
}

func (l *logger) Warnw(msg string, keysAndValues ...any) {
	l.zlogger.Sugar().Warnw(msg, keysAndValues...)
}

func (l *logger) Error(msg string, fields ...Field) {
	l.zlogger.Error(msg, fields...)
}

func (l *logger) Errorf(format string, v ...any) {
	l.zlogger.Sugar().Errorf(format, v)
}

func (l *logger) Errorw(msg string, keysAndValues ...any) {
	l.zlogger.Sugar().Errorw(msg, keysAndValues...)
}

func (l *logger) Panic(msg string, fields ...Field) {
	l.zlogger.Panic(msg, fields...)
}

func (l *logger) Panicf(format string, v ...any) {
	l.zlogger.Sugar().Panicf(format, v)
}

func (l *logger) Panicw(msg string, keysAndValues ...any) {
	l.zlogger.Sugar().Panicw(msg, keysAndValues...)
}

func (l *logger) Fatal(msg string, fields ...Field) {
	l.zlogger.Fatal(msg, fields...)
}

func (l *logger) Fatalf(format string, v ...any) {
	l.zlogger.Sugar().Fatalf(format, v)
}

func (l *logger) Fatalw(msg string, keysAndValues ...any) {
	l.zlogger.Sugar().Fatalw(msg, keysAndValues...)
}

func Info(msg string, fields ...Field) {
	std.zlogger.Info(msg, fields...)
}

func Infof(format string, v ...any) {
	std.zlogger.Sugar().Infof(format, v...)
}

func Infow(msg string, keysAndValues ...any) {
	std.zlogger.Sugar().Infow(msg, keysAndValues...)
}

func Debug(msg string, fields ...Field) {
	std.zlogger.Debug(msg, fields...)
}

func Debugf(format string, v ...any) {
	std.zlogger.Sugar().Debugf(format, v...)
}

func Debugw(msg string, keysAndValues ...any) {
	std.zlogger.Sugar().Debugw(msg, keysAndValues...)
}

func Warn(msg string, fields ...Field) {
	std.zlogger.Warn(msg, fields...)
}

func Warnf(format string, v ...any) {
	std.zlogger.Sugar().Warnf(format, v...)
}

func Warnw(msg string, keysAndValues ...any) {
	std.zlogger.Sugar().Warnw(msg, keysAndValues...)
}

func Error(msg string, fields ...Field) {
	std.zlogger.Error(msg, fields...)
}

func Errorf(format string, v ...any) {
	std.zlogger.Sugar().Errorf(format, v...)
}

func Errorw(msg string, keysAndValues ...any) {
	std.zlogger.Sugar().Errorw(msg, keysAndValues...)
}

func Panic(msg string, fields ...Field) {
	std.zlogger.Panic(msg, fields...)
}

func Panicf(format string, v ...any) {
	std.zlogger.Sugar().Panicf(format, v...)
}

func Panicw(msg string, keysAndValues ...any) {
	std.zlogger.Sugar().Panicw(msg, keysAndValues...)
}

func Fatal(msg string, fields ...Field) {
	std.zlogger.Fatal(msg, fields...)
}

func Fatalf(format string, v ...any) {
	std.zlogger.Sugar().Fatalf(format, v...)
}

func Fatalw(msg string, keysAndValues ...any) {
	std.zlogger.Sugar().Fatalw(msg, keysAndValues...)
}
