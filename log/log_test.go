package log

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
)

func TestEnabledLogger(t *testing.T) {
	b := new(bytes.Buffer)
	l := &Logger{
		logger: log.New(b, "TEST DEBUG ", log.LstdFlags),
		Enable: true,
	}

	l.Debug("logging", "hoge")

	if b.Len() == 0 {
		t.Error("Logger did not write record")
	}

	fmt.Println(b)
	if strings.Contains(b.String(), "TEST DEBUG") == false {
		t.Error("Incorrect output")
	}
}
