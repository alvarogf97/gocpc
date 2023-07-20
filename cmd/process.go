package cmd

import (
	"sync"

	"github.com/alvarogf97/gocpc/pkg/cpc"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process csv file and request CPC for every document record found in it",
	Long:  "Process csv file and request CPC for every document record found in it",
	Run: func(cmd *cobra.Command, args []string) {

		inputFile, _ := cmd.Flags().GetString("input")
		outputFile, _ := cmd.Flags().GetString("output")
		logFile, _ := cmd.Flags().GetString("log")
		threads, _ := cmd.Flags().GetInt("threads")
		startRow, _ := cmd.Flags().GetInt("starts-from")
		retries, _ := cmd.Flags().GetInt("request-retries")

		streams, lines := cpc.CpcStreamCsvReader(inputFile, threads, startRow)
		matches, errors, updates := cpc.ThreadCPCRequester(streams, retries)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			cpc.CpcStreamCsvWriter(outputFile, logFile, matches, errors)
		}()

		bar := progressbar.Default(int64(lines))
		for range updates {
			bar.Add(1)
		}

		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
	processCmd.Flags().StringP("input", "i", "data.csv", "input file")
	processCmd.Flags().StringP("output", "o", "matches.csv", "output file")
	processCmd.Flags().StringP("log", "l", "errors.log", "errors log file")
	processCmd.Flags().IntP("threads", "t", 1, "threads to use")
	processCmd.Flags().IntP("starts-from", "s", 1, "starts from")
	processCmd.Flags().IntP("request-retries", "r", 1, "times failed request will be retried")
}
