package boltdb

import (
	"github.com/asdine/storm"
)

type BoltInterface interface {
	Name() string
}

var db *Bolt

// Bolt is a wrapper around boltdb
type Bolt struct {
	db *storm.DB
}

// NewStorage creates a new BoltDB storage for service promises
func NewStorage(path string) error {
	if db == nil {
		if bolt, err := openDB(path); err == nil {
			db = bolt
		} else {
			return err
		}
	}
	return nil
}

func GetStorage() *Bolt {
	return db
}

// openDB creates new or open existing BoltDB
func openDB(name string) (*Bolt, error) {
	db, err := storm.Open(name)
	return &Bolt{db}, err
}

// Store allows to keep struct grouped by the bucket
func (b *Bolt) Save(object BoltInterface) error {
	return b.db.From(object.Name()).Save(object)
}

// GetAllFrom allows to get all structs from the bucket
func (b *Bolt) All(object BoltInterface) error {
	return b.db.From(object.Name()).All(object)
}

// Delete removes the given struct from the given bucket
func (b *Bolt) Delete(object BoltInterface) error {
	return b.db.From(object.Name()).DeleteStruct(object)
}

// Update allows to update the struct in the given bucket
func (b *Bolt) Update(object BoltInterface) error {
	return b.db.From(object.Name()).Update(object)
}

// Close closes database
func (b *Bolt) Close() error {
	return b.db.Close()
}
