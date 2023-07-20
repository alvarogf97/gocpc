package reader

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
)

type CsvRow struct {
	Row      int
	Document string
	Id       int
}

// Reads the given io.Reader writting the content into a readable channels in a goroutine.
func StreamChannelReader[T any](reader io.ReadSeeker, streams int, start int, formatter func(int, []string) T) ([]chan T, int) {
	lines := lineCounter(reader) - start
	if lines < 0 {
		panic("Not enough lines")
	}
	reader.Seek(0, io.SeekStart)
	channels := make([]chan T, streams)
	for i := range channels {
		channels[i] = make(chan T)
	}

	go func() {
		row := 0
		parser := csv.NewReader(reader)
		defer func() {
			for i := range channels {
				close(channels[i])
			}
		}()

		for {
			record, err := parser.Read()

			if err == io.EOF {
				break
			}
			if err != nil {
				panic(fmt.Sprintf("%s | Stop at row %d", err, row))
			}
			row += 1
			if row < start {
				continue
			}
			channels[row%streams] <- formatter(row, record)
		}
	}()

	return channels, lines
}

func lineCounter(r io.Reader) int {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count

		case err != nil:
			panic(err)
		}
	}
}
