package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	// STEP 5-1: uncomment this line
	// _ "github.com/mattn/go-sqlite3"
)

var errImageNotFound = errors.New("image not found")

type Items struct {
	Items []*Item `json:items`
}

type Item struct {
	ID       int    `db:"id" json:"-"`
	Name     string `db:"name" json:"name"`
	Category string `db:"category" json:"category"`
}

// Please run `go generate ./...` to generate the mock implementation
// ItemRepository is an interface to manage items.
//
//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -package=${GOPACKAGE} -destination=./mock_$GOFILE
type ItemRepository interface {
	Insert(ctx context.Context, item *Item) error
	InsertToFile(ctx context.Context, item *Item) error
}

// itemRepository is an implementation of ItemRepository
type itemRepository struct {
	// fileName is the path to the JSON file storing items.
	fileName string
}

// NewItemRepository creates a new itemRepository.
func NewItemRepository() ItemRepository {
	return &itemRepository{fileName: "items.json"}
}

// Insert inserts an item into the repository.
func (i *itemRepository) Insert(ctx context.Context, item *Item) error {
	// STEP 4-2: add an implementation to store an item

	return nil
}

// Insert inserts an item into the file.
func (i *itemRepository) InsertToFile(ctx context.Context, item *Item) error {
	var items Items

	// checks if file exists
	if _, err := os.Stat(i.fileName); err == nil {
		f, err := os.Open(i.fileName)
		if err != nil {
			return err
		}

		defer f.Close()

		// Decode existing items
		if err := json.NewDecoder(f).Decode(&items); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		items.Items = []*Item{}
	} else {
		return err
	}

	slog.Info("file, items check before inserting")

	// append new Items
	items.Items = append(items.Items, item)

	// Marchal items to JSON
	b, err := json.Marshal(items)

	if err != nil {
		return err
	}

	// Open or create file
	f, err := os.Create(i.fileName)
	if err != nil {
		return err
	}

	defer f.Close()

	// Write items to file
	_, err = f.Write(b)

	if err != nil {
		return err
	}

	slog.Info("item is inserted successfully")

	return nil
}

// StoreImage stores an image and returns an error if any.
// This package doesn't have a related interface for simplicity.
func StoreImage(fileName string, image []byte) error {
	// STEP 4-4: add an implementation to store an image

	return nil
}
