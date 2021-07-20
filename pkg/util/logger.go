package util

import (
	"fmt"
	"github.com/fatih/color"
	"sync"
	"time"
)

const (
	// LevelError 錯誤
	LevelError = iota
	// LevelWarning 警告
	LevelWarning
	// LevelInformational 提示
	LevelInformational
	// LevelDebug 除錯
	LevelDebug
)

var GloablLogger *Logger
var Level = LevelDebug

// Logger 日誌
type Logger struct {
	level int
	mu    sync.Mutex
}

// 日誌顏色
var colors = map[string]func(a ...interface{}) string{
	"Warning": color.New(color.FgYellow).Add(color.Bold).SprintFunc(),
	"Panic":   color.New(color.BgRed).Add(color.Bold).SprintFunc(),
	"Error":   color.New(color.FgRed).Add(color.Bold).SprintFunc(),
	"Info":    color.New(color.FgCyan).Add(color.Bold).SprintFunc(),
	"Debug":   color.New(color.FgWhite).Add(color.Bold).SprintFunc(),
}

// 不同級別前綴與時間的間隔，保持寬度一致
var spaces = map[string]string{
	"Warning": "",
	"Panic":   "  ",
	"Error":   "  ",
	"Info":    "   ",
	"Debug":   "  ",
}

// Println 列印
func (ll *Logger) Println(prefix string, msg string) {
	// TODO Release時去掉
	// color.NoColor = false

	c := color.New()

	ll.mu.Lock()
	defer ll.mu.Unlock()

	_, _ = c.Printf(
		"%s%s %s %s\n",
		colors[prefix]("["+prefix+"]"),
		spaces[prefix],
		time.Now().Format("2006-01-02 15:04:05"),
		msg,
	)
}

// Panic 極端錯誤
func (ll *Logger) Panic(format string, v ...interface{}) {
	if LevelError > ll.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	ll.Println("Panic", msg)
	panic(msg)
}

// Error 錯誤
func (ll *Logger) Error(format string, v ...interface{}) {
	if LevelError > ll.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	ll.Println("Error", msg)
}

// Warning 警告
func (ll *Logger) Warning(format string, v ...interface{}) {
	if LevelWarning > ll.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	ll.Println("Warning", msg)
}

// Info 訊息
func (ll *Logger) Info(format string, v ...interface{}) {
	if LevelInformational > ll.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	ll.Println("Info", msg)
}

// Debug 校驗
func (ll *Logger) Debug(format string, v ...interface{}) {
	if LevelDebug > ll.level {
		return
	}
	msg := fmt.Sprintf(format, v...)
	ll.Println("Debug", msg)
}

// Print GORM 的 Logger實現
//func (ll *Logger) Print(v ...interface{}) {
//	if LevelDebug > ll.level {
//		return
//	}
//	msg := fmt.Sprintf("[SQL] %s", v...)
//	ll.Println(msg)
//}

// BuildLogger 構建logger
func BuildLogger(level string) {
	intLevel := LevelError
	switch level {
	case "error":
		intLevel = LevelError
	case "warning":
		intLevel = LevelWarning
	case "info":
		intLevel = LevelInformational
	case "debug":
		intLevel = LevelDebug
	}
	l := Logger{
		level: intLevel,
	}
	GloablLogger = &l
}

// Log 返回日誌物件
func Log() *Logger {
	if GloablLogger == nil {
		l := Logger{
			level: Level,
		}
		GloablLogger = &l
	}
	return GloablLogger
}
