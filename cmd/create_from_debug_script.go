package cmd

import (
	"context"
	"errors"
	"github.com/arangodb/go-driver"
	"github.com/neunhoef/smart-graph-maker/pkg/parser"
	"github.com/spf13/cobra"
)

var (
	cmdCreateFromDebugScript = &cobra.Command{
		Use:   "debugscript",
		Short: "Create database from files which were generated by the debug script",
		RunE:  createFromDebugScript,
	}
)

func init() {
	var sizeFilename, countFilename string
	var oneshard bool

	cmdCreateFromDebugScript.Flags().StringVar(&sizeFilename, "sizefile", "",
		"File which contains size (in bytes) of shards")
	cmdCreateFromDebugScript.Flags().StringVar(&countFilename, "countfile", "",
		"File which contains number of documents in shards")
	cmdCreateFromDebugScript.Flags().BoolVar(&oneshard, "oneshard", false, "If database should be oneshard type")
}

func createFromDebugScript(cmd *cobra.Command, _ []string) error {
	sizeFilename, _ := cmd.Flags().GetString("sizefile")
	countFilename, _ := cmd.Flags().GetString("countfile")
	oneshard, _ := cmd.Flags().GetBool("oneshard")

	if len(sizeFilename) == 0 {
		return errors.New("file with the size should be provided --sizefile")
	}

	if len(countFilename) == 0 {
		return errors.New("file with the count should be provided --countfile")
	}

	dataFromDebugScript := parser.DatabaseMetaDataFromDebugScript{
		SizeFileName:  sizeFilename,
		CountFileName: countFilename,
	}

	metadata := parser.NewDatabaseMetaData(&dataFromDebugScript)
	if err := metadata.GetData(); err != nil {
		return err
	}

	//metadata.Print()

	options := driver.CreateDatabaseOptions{}
	if oneshard {
		options.Options.Sharding = driver.DatabaseShardingSingle
	}

	return metadata.CreateDatabases(context.Background(), _client, &options)
}
