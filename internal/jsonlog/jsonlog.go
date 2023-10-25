package jsonlog

import (
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// Level 定义级别类型，以表示日志条目的严重性级别。
type Level int8

const (
	LevelInfo Level = iota
	LevelDebug
	LevelWarning
	LevelError
	LevelFatal
	LevelOff
)

// 返回一个人性化的严重程度字符串。
func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

// Logger 定义自定义日志记录器类型。它包含日志条目将被写入的输出目标、日志条目将被写入的最低严重级别，以及用于协调写入的互斥器。
type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

// New 返回一个新的日志记录器实例，该实例会将严重程度达到或超过最低严重程度的日志条目写入特定的输出目标。
func New(out io.Writer, minLevel Level) *Logger {
	return &Logger{
		out:      out,
		minLevel: minLevel,
	}
}

func (l *Logger) print(level Level, message string, properties map[string]string) (int, error) {
	// 如果日志条目的严重性级别低于日志记录器的最低严重性级别，则返回，不做进一步操作。
	if level < l.minLevel {
		return 0, nil
	}
	aux := struct {
		Level      string            `json:"level"`
		Time       string            `json:"time"`
		Message    string            `json:"message"`
		Properties map[string]string `json:"properties,omitempty"`
		Trace      string            `json:"trace,omitempty"`
	}{
		Level:      level.String(),
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Properties: properties,
	}

	// 为 ERROR 和 FATAL 级别的条目提供堆栈跟踪。
	if level >= LevelError {
		aux.Trace = string(debug.Stack())
	}

	// 声明一个行变量，用于保存实际日志条目文本。
	var line []byte
	// 将匿名 struct 保存为 JSON 格式，并将其存储在 line 变量中。如果在创建 JSON 时出现问题，则将日志条目的内容设置为纯文本错误信息。
	line, err := json.Marshal(aux)
	if err != nil {
		line = []byte(LevelError.String() + ": unable to marshal log message: " + err.Error())
	}

	// 锁定互斥，这样就不会同时发生两次写入输出目标的操作。如果我们不这样做，输出中就有可能出现两个或多个日志条目的文本混在一起的情况。
	l.mu.Lock()
	defer l.mu.Unlock()

	// 写入日志条目，后加换行符。
	return l.out.Write(append(line, '\n'))
}

// PrintInfo 声明一些辅助方法，用于编写不同级别的日志条目。请注意，这些方法都接受一个映射作为第二个参数，该映射可包含希望出现在日志条目中的任意 "属性"。
func (l *Logger) PrintInfo(message string, properties map[string]string) {
	l.print(LevelInfo, message, properties)
}

func (l *Logger) PrintDebug(message string, properties map[string]string) {
	l.print(LevelDebug, message, properties)
}

func (l *Logger) PrintWarning(message string, properties map[string]string) {
	l.print(LevelWarning, message, properties)
}

func (l *Logger) PrintError(err error, properties map[string]string) {
	l.print(LevelError, err.Error(), properties)
}

// PrintFatal 对于 FATAL 级别的条目，我们也会终止应用程序。
func (l *Logger) PrintFatal(err error, properties map[string]string) {
	l.print(LevelFatal, err.Error(), properties)
	os.Exit(1)
}

// 我们还在日志类型上实现了 Write() 方法，使其满足 io.Writer 接口的要求。该方法会写入一个 ERROR 级别的日志条目，但没有附加属性。
func (l *Logger) Write(message []byte) (n int, err error) {
	return l.print(LevelError, string(message), nil)
}
