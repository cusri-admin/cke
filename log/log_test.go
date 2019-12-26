package log

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func setStdIO(out io.Writer) {
	loggers[ERROR] = newLogger(out, ERROR)
	loggers[WARNING] = newLogger(out, WARNING)
	loggers[INFO] = newLogger(out, INFO)
	loggers[DEBUG] = newLogger(out, DEBUG)
}

func TestLevel(t *testing.T) {
	var f bytes.Buffer
	setStdIO(&f)

	SetLevel(DEBUG)

	Debugf("%s", "debug")
	if x := f.String(); !strings.Contains(x, "debug") {
		t.Errorf("Expected debug log to be %s, got %s", "debug", x)
	}

	f.Reset()

	SetLevel(INFO)

	Debug("debug")
	if x := f.String(); x != "" {
		t.Errorf("Expected no level logs, got %s", x)
	}
}

func TestLevels(t *testing.T) {
	var f bytes.Buffer
	const ts = "test"
	setStdIO(&f)

	SetLevel(DEBUG)

	Debugf("%s", "debug")
	if x := f.String(); !strings.Contains(x, "debug") {
		t.Errorf("Expected debug log to be %s, got %s", "debug", x)
	}
	f.Reset()
	Debug("debug")
	if x := f.String(); !strings.Contains(x, "debug") {
		t.Errorf("Expected debug log to be %s, got %s", "debug", x)
	}

	f.Reset()

	Infof("%s", "info")
	if x := f.String(); !strings.Contains(x, "info") {
		t.Errorf("Expected info log to be %s, got %s", "info", x)
	}
	f.Reset()
	Info("info")
	if x := f.String(); !strings.Contains(x, "info") {
		t.Errorf("Expected info log to be %s, got %s", "info", x)
	}

	f.Reset()

	Warningf("%s", "warning")
	if x := f.String(); !strings.Contains(x, "warning") {
		t.Errorf("Expected warning log to be %s, got %s", "warning", x)
	}
	f.Reset()
	Warning("warning")
	if x := f.String(); !strings.Contains(x, "warning") {
		t.Errorf("Expected warning log to be %s, got %s", "warning", x)
	}

	f.Reset()

	Errorf("%s", "error")
	if x := f.String(); !strings.Contains(x, "error") {
		t.Errorf("Expected error log to be %s, got %s", "error", x)
	}
	f.Reset()
	Error("error")
	if x := f.String(); !strings.Contains(x, "error") {
		t.Errorf("Expected error log to be %s, got %s", "error", x)
	}
}
