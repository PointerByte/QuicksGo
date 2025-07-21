package logger

import "encoding/json"

// new creates the JSON formatter for log entries
func new() formatter {
	return &jsonFormatter{}
}

type jsonFormatter struct{}

// Format serializes the entry as JSON with fields: time, level, msg, and key/value pairs
func (f *jsonFormatter) format(e *entry) ([]byte, error) {
	obj := make(map[string]any)
	obj["time"] = e.Time.Format("2006-01-02T15:04:05.000Z07:00")
	obj["level"] = e.Level
	obj["msg"] = e.Message
	// build a separate map for your dynamic fields
	fields := make(map[string]interface{}, len(e.Fields)/2)
	for i := 0; i < len(e.Fields); i += 2 {
		if key, ok := e.Fields[i].(string); ok && i+1 < len(e.Fields) {
			fields[key] = e.Fields[i+1]
		}
	}
	obj["fields"] = fields
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
