package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	// STEP 5-1: uncomment this line
	_ "github.com/mattn/go-sqlite3"
)

var errImageNotFound = errors.New("image not found")

type Item struct {
	ID        int    `db:"id" json:"-"`
	Name      string `db:"name" json:"name"`
	Category  string `db:"category" json:"category"`
	ImageName string `db:"image_name" json:"image_name"`
}

// Please run `go generate ./...` to generate the mock implementation
// ItemRepository is an interface to manage items.
//
//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -package=${GOPACKAGE} -destination=./mock_$GOFILE
type ItemRepository interface {
	SelectAll(ctx context.Context) ([]*Item, error)
	SelectByID(ctx context.Context, itemID int) (*Item, error)
	Insert(ctx context.Context, item *Item) error
	SearchByName(ctx context.Context, keyWord string) ([]*Item, error)
}

// itemRepository is an implementation of ItemRepository
type itemRepository struct {
	db *sql.DB
	// fileName is the path to the JSON file storing items.
	fileName string
}

// NewItemRepository creates a new itemRepository.
func NewItemRepository(db *sql.DB) ItemRepository {
	return &itemRepository{db: db, fileName: ""}
}

func (i *itemRepository) SelectAll(ctx context.Context) ([]*Item, error) {
	if i.fileName != "" {
		items, err := i.getItemsFromFile(ctx)
		return items, err
	}

	db := i.db

	rows, err := db.Query("SELECT id, name, category, image_name FROM items")
	if err != nil {
		return nil, err
	}

	defer rows.Close() // リソースの解放

	items := []*Item{}

	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.ImageName); err != nil {
			return nil, err
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil

}

func (i *itemRepository) SelectByID(ctx context.Context, itemID int) (*Item, error) {
	if i.fileName != "" {
		item, err := i.getItemFromFileByID(ctx, itemID)
		if err != nil {
			return nil, err
		}
		return item, nil
	}

	var item Item

	db := i.db

	err := db.QueryRow("SELECT id, name, category, image_name FROM items WHERE id = ?", itemID).Scan(&item.ID, &item.Name, &item.Category, &item.ImageName)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}

	return &item, nil
}

func (i *itemRepository) SearchByName(ctx context.Context, keyword string) ([]*Item, error) {
	// db connection
	db := i.db

	// db query
	rows, err := db.Query("SELECT id, name, category, image_name FROM items WHERE name LIKE ?", "%"+keyword+"%")

	if err != nil {
		return nil, err
	}

	// defer rows
	defer rows.Close()

	// result items
	items := []*Item{}

	for rows.Next() {
		var item Item
		err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.ImageName)

		if err != nil {
			return nil, err
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (i *itemRepository) getItemFromFileByID(ctx context.Context, itemID int) (*Item, error) {
	items, err := i.getItemsFromFile(ctx)

	if err != nil {
		return nil, err
	}

	if itemID < 1 || items == nil || itemID > len(items) {
		slog.Error("item not found")
		return nil, errors.New("item not found")
	}

	return items[itemID-1], nil
}

func (i *itemRepository) getItemsFromFile(ctx context.Context) ([]*Item, error) {
	var items []*Item
	// checks if file exists
	if _, err := os.Stat(i.fileName); err == nil {
		f, err := os.Open(i.fileName)
		if err != nil {
			return nil, err
		}

		defer f.Close()

		// Decode existing items
		if err := json.NewDecoder(f).Decode(&items); err != nil {
			return nil, err
		}
	} else if os.IsNotExist(err) {
		items = []*Item{}
	} else {
		return nil, err
	}

	return items, nil

}

// Insert inserts an item into the repository.
func (i *itemRepository) Insert(ctx context.Context, item *Item) error {
	// STEP 4-2: add an implementation to store an item
	if i.fileName != "" {
		return i.InsertToFile(ctx, item)
	}

	db := i.db

	_, err := db.Exec("INSERT INTO items (name, category, image_name) VALUES (?, ?, ?)", item.Name, item.Category, item.ImageName)

	return err

}

// Insert inserts an item into the file.
func (i *itemRepository) InsertToFile(ctx context.Context, item *Item) error {
	var items []*Item

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
		items = []*Item{}
	} else {
		return err
	}

	slog.Info("file, items check before inserting")

	// append new Items
	items = append(items, item)

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
