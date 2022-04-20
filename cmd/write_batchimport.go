package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/arangodb/go-driver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/neunhoef/collectionmaker/pkg/database"
	"sort"
	"sync"
	"time"
)

var (
	cmdWriteBatchImport = &cobra.Command{
		Use:   "batchimport",
		Short: "Write batchimport",
		RunE:  writeBatchImport,
	}
)

type Doc struct {
	Key           string `json:"_key"`
	Sha           string `json:"sha"`
	Payload       string `json:"payload"`
}

func init() {
	var parallelism int = 1
	var startDelay int64 = 5
	var number int64 = 1000000
	var payloadSize int64 = 10
	var batchSize int64 = 10000
	cmdWriteBatchImport.Flags().IntVar(&parallelism, "parallelism", parallelism, "set -parallelism to use multiple go routines")
	cmdWriteBatchImport.Flags().Int64Var(&number, "number", number, "set -number for number of edges to write per go routine")
	cmdWriteBatchImport.Flags().Int64Var(&startDelay, "start-delay", startDelay, "Delay between the start of two go routines.")
	cmdWriteBatchImport.Flags().Int64Var(&payloadSize, "payload-size", payloadSize, "Size in bytes of payload in each document.")
	cmdWriteBatchImport.Flags().Int64Var(&batchSize, "batch-size", batchSize, "Size in number of documents of each import batch.")
}

// writeBatchImport writes edges in parallel
func writeBatchImport(cmd *cobra.Command, _ []string) error {
	parallelism, _ := cmd.Flags().GetInt("parallelism")
	number, _ := cmd.Flags().GetInt64("number")
	startDelay, _ := cmd.Flags().GetInt64("start-delay")
	payloadSize, _ := cmd.Flags().GetInt64("payload-size")
	batchSize, _ := cmd.Flags().GetInt64("batch-size")

	db, err := _client.Database(context.Background(), "_system")
	if err != nil {
		return errors.Wrapf(err, "can not get database: %s", "_system")
	}

	if err := writeSomeBatchesParallel(parallelism, number, startDelay, payloadSize, batchSize, db); err != nil {
		return errors.Wrapf(err, "can not do some batch imports")
	}

	return nil
}

// writeSomeBatchesParallel does some batch imports in parallel
func writeSomeBatchesParallel(parallelism int, number int64, startDelay int64, payloadSize int64, batchSize int64, db driver.Database) error {
	var mutex sync.Mutex
	totaltimestart := time.Now()
	wg := sync.WaitGroup{}
	haveError := false
	for i := 1; i <= parallelism; i++ {
	  time.Sleep(time.Duration(startDelay) * time.Millisecond)
		i := i // bring into scope
		wg.Add(1)

		go func(wg *sync.WaitGroup, i int) {
			defer wg.Done()
			fmt.Printf("Starting go routine...\n")
			err := writeSomeBatches(number, int64(i), payloadSize, batchSize, db, &mutex)
			if err != nil {
				fmt.Printf("writeSomeBatches error: %v\n", err)
				haveError = true
			}
			mutex.Lock()
			fmt.Printf("Go routine %d done\n", i)
			mutex.Unlock()
		}(&wg, i)
	}

	wg.Wait()
	totaltimeend := time.Now()
	totaltime := totaltimeend.Sub(totaltimestart)
	docspersec := float64(int64(parallelism) * number * batchSize) / (float64(totaltime) / float64(time.Second))
	fmt.Printf("\nTotal number of documents written: %d, total time: %v, total edges per second: %f\n", int64(parallelism) * number * batchSize, totaltimeend.Sub(totaltimestart), docspersec)
	if !haveError {
		return nil
	}
	fmt.Printf("Error in writeSomeBatches.\n")
	return fmt.Errorf("Error in writeSomeBatches.")
}

// writeSomeBatches writes `nrBatches` batches with `batchSize` documents.
func writeSomeBatches(nrBatches int64, id int64, payloadSize int64, batchSize int64, db driver.Database, mutex *sync.Mutex) error {
	edges, err := db.Collection(nil, "batchimport")
	if err != nil {
		fmt.Printf("writeSomeBatches: could not open `batchimport` collection: %v\n", err)
		return err
	}
	docs := make([]Doc, 0, batchSize)
  times := make([]time.Duration, 0, batchSize)
	cyclestart := time.Now()
	for i := int64(1); i <= nrBatches; i++ {
		start := time.Now()
    for j := int64(1); j <= batchSize; j++ {
			x := fmt.Sprintf("%d", (id * nrBatches + i - 1) * batchSize + j)
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(x)))
			x = "SHA" + x
			sha := fmt.Sprintf("%x", sha256.Sum256([]byte(x)))
			pay := database.MakeRandomString(int(payloadSize))
			docs = append(docs, Doc{
				Key: key, Sha: sha, Payload: pay })
	  }
		ctx, cancel := context.WithTimeout(driver.WithOverwriteMode(context.Background(), driver.OverwriteModeIgnore), time.Hour)
		_, _, err := edges.CreateDocuments(ctx, docs)
		cancel()
		if err != nil {
			fmt.Printf("writeSomeBatches: could not write batch: %v\n", err)
			return err
		}
		docs = docs[0:0]
    times = append(times, time.Now().Sub(start))
		if i % 100 == 0 {
			mutex.Lock()
			fmt.Printf("%s Have imported %d batches for id %d.\n", time.Now(), int(i), id)
			mutex.Unlock()
		}
	}
	sort.Sort(DurationSlice(times))
	var sum int64 = 0
	for _, t := range times {
		sum = sum + int64(t)
	}
	totaltime := time.Now().Sub(cyclestart)
	nrDocs := batchSize * nrBatches
	docspersec := float64(nrDocs) / (float64(totaltime) / float64(time.Second))
	mutex.Lock()
	fmt.Printf("Times for %d batches: %s (median), %s (90%%ile), %s (99%%ilie), %s (average), docs per second in this go routine: %f\n", nrBatches, times[int(float64(0.5) * float64(nrBatches))], times[int(float64(0.9) * float64(nrBatches))], times[int(float64(0.99) * float64(nrBatches))], time.Duration(sum / nrBatches), docspersec)
	mutex.Unlock()
	return nil
}
