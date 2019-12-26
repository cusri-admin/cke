package log

import (
	"io"
	"os"
	"runtime"
	"time"

	"github.com/mattn/go-isatty"
)

var (
	//	green   = []byte{27, 91, 51, 52, 109}
	white  = []byte{27, 91, 51, 57, 109}
	yellow = []byte{27, 91, 51, 51, 109}
	red    = []byte{27, 91, 51, 49, 109}
	blue   = []byte{27, 91, 51, 54, 109}
	//	magenta = []byte{27, 91, 51, 55, 109}
	//	cyan    = []byte{27, 91, 51, 56, 109}
	reset = []byte{27, 91, 48, 109}
)

type Logger struct {
	out         io.Writer // destination for output
	level       Level
	statusColor []byte
	isTerm      bool
}

func newLogger(out io.Writer, level Level) *Logger {
	var isTerm bool
	var statusColor []byte
	if w, ok := out.(*os.File); !ok ||
		(os.Getenv("TERM") == "dumb" || (!isatty.IsTerminal(w.Fd()) && !isatty.IsCygwinTerminal(w.Fd()))) {
		isTerm = false
	} else {
		isTerm = true
		switch level {
		case DEBUG:
			statusColor = blue
			break
		case INFO:
			statusColor = white
			break
		case WARNING:
			statusColor = yellow
			break
		case ERROR:
			statusColor = red
			break
		default:
			isTerm = false
			break
		}
	}

	return &Logger{
		level:       level,
		out:         out,
		isTerm:      isTerm,
		statusColor: statusColor,
	}
}

func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

func formatTimeHeader(buf *[]byte, t time.Time) {

	//day
	year, month, day := t.Date()
	itoa(buf, year, 4)
	*buf = append(*buf, '/')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '/')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')

	//time
	hour, min, sec := t.Clock()
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, min, 2)
	*buf = append(*buf, ':')
	itoa(buf, sec, 2)
	*buf = append(*buf, ' ')
}

func formatFileHeader(buf *[]byte, file string, line int) {
	// file name and line
	//if l.flag&Lshortfile != 0 {
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short
	//}

	*buf = append(*buf, file...)
	*buf = append(*buf, ':')
	itoa(buf, line, -1)
	*buf = append(*buf, ": "...)
}

func (l *Logger) output(s string) error {

	var buf []byte

	buf = append(buf, '[')
	if l.isTerm {
		buf = append(buf, l.statusColor...)
	}
	buf = append(buf, l.level.String()...)
	if l.isTerm {
		buf = append(buf, reset...)
	}
	buf = append(buf, ']')
	buf = append(buf, ' ')

	now := time.Now() // get this early.
	formatTimeHeader(&buf, now)
	var file string
	var line int
	//if l.flag&(Lshortfile|Llongfile) != 0 {
	var ok bool
	_, file, line, ok = runtime.Caller(3)
	if !ok {
		file = "???"
		line = 0
	}
	//}
	formatFileHeader(&buf, file, line)
	buf = append(buf, s...)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		buf = append(buf, '\n')
	}
	_, err := l.out.Write(buf)
	return err
}
