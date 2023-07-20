package cpc

import (
	"fmt"
	"os"
	"sync"

	"github.com/alvarogf97/gocpc/pkg/reader"
	"github.com/alvarogf97/gocpc/pkg/writer"
)

type CpcCsvRow struct {
	Row      int
	Document string
}

func cpcCsvRowFromRecord(row int, record []string) CpcCsvRow {
	return CpcCsvRow{
		Row:      row,
		Document: record[0],
	}
}

func recordFromCpcResponse(record CpcRecord) string {
	return fmt.Sprintf("%s,%s,%t,%t,%t", record.Name, record.Document, record.Administrator, record.Disabled, record.Debtor)
}

func CpcStreamCsvReader(filename string, streams int, startRow int) ([]chan CpcCsvRow, int) {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	return reader.StreamChannelReader(file, streams, startRow, cpcCsvRowFromRecord)
}

func CpcStreamCsvWriter(filename string, logFilename string, stream chan CpcRecord, errors chan string) {
	headers := "Name,document,administrator,disabled,debtor\n"
	var wg sync.WaitGroup

	outputFile, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	logFile, err := os.Create(logFilename)
	if err != nil {
		panic(err)
	}

	defer outputFile.Close()
	defer logFile.Close()

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer.StreamChannelWriter(outputFile, headers, stream, recordFromCpcResponse)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer.StreamChannelWriter(logFile, "", errors, writer.SimpleStringFormater)
	}()

	wg.Wait()
}
