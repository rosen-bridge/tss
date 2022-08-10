package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const (
	// DPanic, Panic and Fatal level can not be set by user
	DebugLevelStr   string = "debug"
	InfoLevelStr    string = "info"
	WarningLevelStr string = "warning"
	ErrorLevelStr   string = "error"
)

var (
	globalLogger *zap.Logger
)

// call it in defer
func Sync() error {
	return globalLogger.Sync()
}

func Init(logLevel string, logFile string, dev bool) error {

	var level zapcore.Level
	switch logLevel {
	case DebugLevelStr:
		level = zap.DebugLevel
	case InfoLevelStr:
		level = zap.InfoLevel
	case WarningLevelStr:
		level = zap.WarnLevel
	case ErrorLevelStr:
		level = zap.ErrorLevel
	default:
		return fmt.Errorf("unknown log level %s", logLevel)
	}

	ws := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    1, //MB
		MaxBackups: 30,
		MaxAge:     90, //days
		Compress:   false,
	})

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	//developmentEncoder := zap.NewDevelopmentEncoderConfig()
	//developmentEncoder.EncodeLevel = zapcore.CapitalColorLevelEncoder
	//productionEncoder := zap.NewProductionEncoderConfig()
	//productionEncoder.EncodeLevel = zapcore.CapitalColorLevelEncoder

	core := zapcore.NewCore(
		// use NewConsoleEncoder for human-readable output
		//zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.NewJSONEncoder(encoderConfig),
		// write to stdout as well as log files
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), ws),

		zap.NewAtomicLevelAt(level),
	)
	var _globalLogger *zap.Logger
	if dev {
		_globalLogger = zap.New(core, zap.AddCaller(), zap.Development())
	} else {
		_globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	}
	zap.ReplaceGlobals(_globalLogger)
	globalLogger = _globalLogger
	return nil
}

func NewSugar(name string) *zap.SugaredLogger {
	return globalLogger.Named(name).Sugar()
	//logger, _ := zap.NewProduction()
	//return logger.Sugar()
}
