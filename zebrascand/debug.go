package main

import (
	"encoding/hex"
	"fmt"
	log "github.com/sirupsen/logrus"
)

type debugTextFormatter struct {
	next log.Formatter
}

func (self *debugTextFormatter) Format(entry *log.Entry) ([]byte, error) {
	delay := make(map[string][]byte)
	for name, value := range entry.Data {
		switch data := value.(type) {
		case []byte:
			delay[name] = data
			delete(entry.Data, name)
		}
	}
	var res []byte
	res, err := self.next.Format(entry)
	if err != nil {
		return res, err
	}

	for name, value := range delay {
		res = append(res, []byte(fmt.Sprintf("  %s (%d bytes):\n", name, len(value)))...)

		lineStart := true
		for _, c := range []byte(hex.Dump(value)) {
			if lineStart && c != '\n' {
				res = append(res, ' ', ' ', ' ', ' ')
			}
			res = append(res, c)
			lineStart = (c == '\n')
		}
	}

	return res, err
}
