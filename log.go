package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"

	"log"
	"time"
)

type LogOptions uint8

const (
	LogToStdout LogOptions = 1 << iota
	LogToFile
	ReshapeLogs
	DefaultLogOptions = LogToStdout | LogToFile | ReshapeLogs
)

var level Level = TraceL

func any(lhs string, rhs []string) bool {
	for _, item := range rhs {
		if item == lhs {
			return true
		}
	}
	return false
}

// SetLogLevel tries to parse the given string to figure out the desired log
// level for the application
func SetLogLevel(l string) {
	if any(l, []string{"information", "info", "i", "in"}) {
		level = Information
	} else if any(l, []string{"warning", "warn", "w", "wa"}) {
		level = Warning
	} else if any(l, []string{"error", "err", "er", "e"}) {
		level = ErrorL
	} else if any(l, []string{"debug", "deb", "de", "d"}) {
		level = DebugL
	} else {
		level = TraceL
	}
}

// file is essentially a wrapper to satisfy the io.Writer interface by using
// Write to handle file opening and closing operations
type file struct {
	Name string
}

type Logger struct {
	loggers []io.Writer
	options LogOptions
}

func (f *file) Write(p []byte) (n int, err error) {
	n = len(p)
	return n, f.WriteMsg(string(p))
}

// WriteMsg is the internal wrapper for the interface satisfying of the logging
// functionality
func (f *file) WriteMsg(msg string, args ...interface{}) error {
	perms := os.O_APPEND | os.O_WRONLY | os.O_CREATE
	file, err := os.OpenFile(f.Name, perms, os.ModeAppend)
	defer file.Close()
	if err == nil {
		if _, err := fmt.Fprintf(file, msg+"\n", args...); err != nil {
			fmt.Fprintln(os.Stdout, err.Error())
		} else {
			return nil
		}
	} else if os.IsNotExist(err) {
		re := regexp.MustCompile("[A-Za-z0-9." + dirDelimit + "]+" + dirDelimit)
		dirPath := re.FindString(f.Name)
		if err = os.MkdirAll(dirPath, 0744); err == nil {
			file, err = os.OpenFile(f.Name, perms, os.ModeAppend)
		}
		if os.IsExist(err) {
			panic(err)
		} else {
			errMsg := fmt.Sprintf("Could not open log file: %v", err)
			panic(errMsg)
		}
	} else {
		err = fmt.Errorf("Could not log to file: %v", err)
		fmt.Fprintln(os.Stdout, err.Error())
		return err
	}
	// From line 39
	if err != nil {
		fmt.Fprintln(os.Stdout, err.Error())
	}
	if _, err := fmt.Fprintf(file, msg+"\n", args...); err != nil {
		fmt.Fprintln(os.Stdout, err.Error())
	}
	return nil
}

// Level of the desired log
type Level int

const (
	// TraceL is a trace level log
	TraceL = iota
	// Information level log [I]
	Information
	// DebugL level log [D]
	DebugL
	// Warning level log [W]
	Warning
	// ErrorL is an error level log renamed to avoid naming conflict
	ErrorL

	// TextMaxWidth is the maximum number of characters that are allowed to be entered into logs
	TextMaxWidth = 100
)

func NewDefault(logFile string) *Logger {
	return New(logFile, DefaultLogOptions)
}

// New returns a new instance of a logger object on demand
func New(logFile string, options LogOptions) *Logger {
	l := Logger{}
	// Changed because docker-compose logs are really useful
	if runtime.GOOS != "windows" || options&LogToStdout > 0 {
		l.loggers = append(l.loggers, os.Stdout)
	}
	if options&LogToFile > 0 {
		f := &file{
			Name: logFile,
		}
		l.loggers = append(l.loggers, io.Writer(f))
	}
	l.options = options
	return &l
}

func prefix(level Level) string {
	str := time.Now().Format("2006-01-02T15:04:05-0700")
	char := ""
	switch level {
	case TraceL:
		char = "T"
	case Information:
		char = "I"
	case Warning:
		char = "W"
	case DebugL:
		char = "D"
	default:
		char = "E"
	}
	return fmt.Sprintf("%v [%v] -\t", str, char)
}

func (l *Logger) _log(lev Level, format string, args ...interface{}) {
	if len(l.loggers) == 0 {
		panic("Could not log because no loggers are configured")
	}
	if lev < level {
		return
	}
	log.SetOutput(io.MultiWriter(l.loggers...))
	var msg string
	fmsg := fmt.Sprintf(format, args...)
	if l.options&ReshapeLogs > 0 {
		msg = reshape(prefix(lev), fmsg)
	} else {
		msg = prefix(lev) + fmsg
	}
	for _, logger := range l.loggers {
		fmt.Fprintln(logger, msg)
	}
}

// reshape attempts to answer the visual problem of giving a margin to text
// based on the length of the desired prefix. This is so tha the eye level of
// the logs are aligned without having to worry about having to sort through
// the. Assumes ASCII
func reshape(prefix, text string) string {
	leftmargin := len(prefix)
	var (
		words = make([][]byte, 0, len(text))
		_text = []byte(text)
		word  = make([]byte, 0, 15)
		buf   bytes.Buffer
	)
	for i, char := range _text {
		if char == 0x20 || char == 0xA || char == 0xD {
			if len(word) > 0 {
				words = append(words, word)
			}
			word = make([]byte, 0, 15)
			continue
		}
		word = append(word, char)
		if i == len(_text)-1 {
			words = append(words, word)
		}
	}

	// Will likely not grow very often, so safe to give a small header
	buf.Grow(len(text) + 50)

	line := make([]byte, 0, 15)
	initLine := func(linesIndex int) {
		line = make([]byte, 0, 15)
		if linesIndex == 0 {
			return
		}
		for i := 0; i < leftmargin-4; i++ {
			line = append(line, 0x20)
		}
		line = append(line, 0x9)
	}
	initLine(0)
	line = []byte(prefix)
	for i, word := range words {
		if len(word)+len(line) > TextMaxWidth {
			buf.Write(line)
			buf.WriteString(carriageReturn)
			initLine(i)
		}
		if len(line) > 0 {
			line = append(line, 0x20)
		}
		line = append(line, word...)
		if i == len(words)-1 {
			buf.Write(line)
		}
	}

	return buf.String()
}

// Trace issues a log with trace level
func (l *Logger) Trace(fmt string, args ...interface{}) {
	l._log(TraceL, fmt, args...)
}

// Warn issues a log as a warning
func (l *Logger) Warn(fmt string, args ...interface{}) {
	l._log(Warning, fmt, args...)
}

// Info issues a log as information
func (l *Logger) Info(fmt string, args ...interface{}) {
	l._log(Information, fmt, args...)
}

// Debug issues a log as debug information
func (l *Logger) Debug(fmt string, args ...interface{}) {
	l._log(DebugL, fmt, args...)
}

// Error issues a log as an error message
func (l *Logger) Error(fmt string, args ...interface{}) {
	l._log(ErrorL, fmt, args...)
}
