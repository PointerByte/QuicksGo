package logger

import (
	"bytes"
	"fmt"
)

// newTextFormatter creates the default text formatter
func newTextFormatter() formatter {
	return &textFormatter{}
}

type textFormatter struct{}

// Format serializes the entry to text: TIMESTAMP [LEVEL] msg key1=val1 key2=val2
func (f *textFormatter) format(e *entry) ([]byte, error) {
	buf := &bytes.Buffer{}
	timestamp := e.Time.Format("2006-01-02T15:04:05.000Z07:00")
	buf.WriteString(fmt.Sprintf("%s [%s] %s", timestamp, e.Level, e.Message))
	for i := 0; i < len(e.Fields); i += 2 {
		key, ok1 := e.Fields[i].(string)
		val := ""
		if ok1 && i+1 < len(e.Fields) {
			val = fmt.Sprint(e.Fields[i+1])
		}
		buf.WriteString(fmt.Sprintf(" %s=%s", key, val))
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
