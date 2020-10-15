package database

import (
	"context"
	"fmt"
	"github.com/arangodb/go-driver"
	err2 "github.com/pkg/errors"
	"math/rand"
)

// IsNameSystemReserved checks if name of arangod resource is forbidden.
func IsNameSystemReserved(name string) bool {
	if len(name) > 0 && name[0] == '_' {
		return true
	}

	return false
}

// CreateOrGetDatabase returns handle to a database. If database does not exist then it is created.
func CreateOrGetDatabase(ctx context.Context, client driver.Client, DBName string,
	options *driver.CreateDatabaseOptions) (driver.Database, error) {
	if IsNameSystemReserved(DBName) {
		return client.Database(ctx, DBName)
	}

	handle, err := client.CreateDatabase(ctx, DBName, options)
	if err != nil {
		if driver.IsConflict(err) {
			return client.Database(ctx, DBName)
		}
		return nil, err
	}

	return handle, nil
}

// CreateOrGetCollection returns handle to a collection. If collection does not exist then it is created.
func CreateOrGetCollection(ctx context.Context, DBHandle driver.Database, colName string,
	options *driver.CreateCollectionOptions) (driver.Collection, error) {

	colHandle, err := DBHandle.CreateCollection(ctx, colName, options)

	if err == nil {
		return colHandle, nil
	}

	if driver.IsConflict(err) {
		// collection already exists
		return DBHandle.Collection(ctx, colName)
	}

	return nil, err
}

// DocumentGenerator describes the behaviour how to add document.
type DocumentGenerator interface {
	Add(sizeOfDocument int64) interface{} // Add adds new document.
}

type Collection struct {
	expectedSize      int64 // number of bytes for the whole collection
	expectedCount     int64 // number of document to create.
	documentGenerator DocumentGenerator
	colHandle         driver.Collection
}

// NewCollectionCreator creates new collection creator.
func NewCollectionCreator(expectedSize, expectedCount int64, documentGenerator DocumentGenerator,
	colHandle driver.Collection) Collection {

	return Collection{
		expectedSize:      expectedSize,
		expectedCount:     expectedCount,
		documentGenerator: documentGenerator,
		colHandle:         colHandle,
	}
}

// CreateDocuments writes documents using specific document generator.
func (c Collection) CreateDocuments(ctx context.Context) error {

	if c.expectedCount == 0 || c.expectedSize == 0 {
		return nil
	}

	if c.colHandle == nil {
		return fmt.Errorf("collection handler can not be nil")
	}

	currentCount, err := c.colHandle.Count(ctx)
	if err != nil {
		return err2.Wrapf(err, "con not get count of documents for a collection %s", c.colHandle.Name())
	}

	sizeOfEachDocument := c.expectedSize / c.expectedCount

	for c.expectedCount-currentCount > 0 {
		var sentSize int64

		documents := make([]interface{}, 0, c.expectedCount-currentCount)
		var i int64
		for i = 0; i < c.expectedCount-currentCount; i++ {
			documents = append(documents, c.documentGenerator.Add(sizeOfEachDocument))
			sentSize += sizeOfEachDocument
			if sentSize > 100000000 {
				break
			}
		}

		fmt.Printf("\r%s Count: %d/%d", c.colHandle.Name(), currentCount, c.expectedCount)
		currentCount += int64(len(documents))
		if _, _, err = c.colHandle.CreateDocuments(context.Background(), documents); err != nil {
			return err2.Wrap(err, "can not write documents")
		}
		fmt.Printf("\r%s.%s Count: %d/%d", c.colHandle.Database().Name(), c.colHandle.Name(),
			currentCount, c.expectedCount)
		c.colHandle.Database().Name()
	}

	fmt.Printf("\rFinished %s.%s, Count: %d\n", c.colHandle.Database().Name(), c.colHandle.Name(),
		c.expectedCount)

	return nil
}

// DataTest is the example data with one field to write as a one document.
type DataTest struct {
	FirstField string `json:"a,omitempty"`
}

// DocumentWithOneField creates one document with one field.
type DocumentWithOneField struct{}

func (d DocumentWithOneField) Add(sizeOfDocument int64) interface{} {
	return DataTest{
		FirstField: MakeRandomString(int(sizeOfDocument)),
	}
}

// MakeRandomString creates slice of bytes for the provided length.
// Each byte is in range from 33 to 123.
func MakeRandomString(length int) string {
	b := make([]byte, length, length)

	for i := 0; i < length; i++ {
		s := rand.Int()%90 + 33
		b[i] = byte(s)
	}

	return string(b)
}
