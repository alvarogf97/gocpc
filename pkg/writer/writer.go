package writer

import (
	"fmt"
	"io"
)

func SimpleStringFormater(s string) string {
	return s
}

func StreamChannelWriter[T any](writer io.StringWriter, headers string, stream chan T, formatter func(T) string) {
	writer.WriteString(headers)
	for data := range stream {
		writer.WriteString(fmt.Sprintf("%s\n", formatter(data)))
	}
}
