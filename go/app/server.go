package app

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Server struct {
	// Port is the port number to listen on.
	Port string
	// ImageDirPath is the path to the directory storing images.
	ImageDirPath string
	DBPath       string
}

// Run is a method to start the server.
// This method returns 0 if the server started successfully, and 1 otherwise.
func (s Server) Run() int {
	// set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	// STEP 4-6: set the log level to DEBUG
	slog.SetLogLoggerLevel(slog.LevelInfo)

	// set up CORS settings
	frontURL, found := os.LookupEnv("FRONT_URL")
	if !found {
		frontURL = "http://localhost:3000"
	}

	// STEP 5-1: set up the database connection
	db, err := sql.Open("sqlite3", s.DBPath)

	if err != nil {
		slog.Error("Failed to open db")
		return 1
	}

	defer db.Close()

	// set up handlers
	itemRepo := NewItemRepository(db)
	h := &Handlers{imgDirPath: s.ImageDirPath, itemRepo: itemRepo}

	// set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.Hello)
	mux.HandleFunc("GET /items", h.GetItems)
	mux.HandleFunc("GET /items/{item_id}", h.GetItemByID)
	mux.HandleFunc("POST /items", h.AddItem)
	mux.HandleFunc("GET /images/{filename}", h.GetImage)

	// start the server
	slog.Info("http server started on", "port", s.Port)
	err = http.ListenAndServe(":"+s.Port, simpleCORSMiddleware(simpleLoggerMiddleware(mux), frontURL, []string{"GET", "HEAD", "POST", "OPTIONS"}))
	if err != nil {
		slog.Error("failed to start server: ", "error", err)
		return 1
	}

	return 0
}

type Handlers struct {
	// imgDirPath is the path to the directory storing images.
	imgDirPath string
	itemRepo   ItemRepository
}

type HelloResponse struct {
	Message string `json:"message"`
}

// Hello is a handler to return a Hello, world! message for GET / .
func (s *Handlers) Hello(w http.ResponseWriter, r *http.Request) {
	resp := HelloResponse{Message: "Hello, world!"}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type GetItemsResponse struct {
	Items []*Item `json:"items"`
}

type AddItemRequest struct {
	Name     string `form:"name"`
	Category string `form:"category"` // STEP 4-2: add a category field
	Image    []byte `form:"image"`    // STEP 4-4: add an image field
}

type AddItemResponse struct {
	Name      string `json:"name`
	Category  string `json:"category"`
	ImageName string `json:"image_name"`
}

type GetItemByIDRequest struct {
	ItemID int `json:"item_id"`
}

type GetItemByIDResponse struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	ImageName string `json:"image_name"`
}

// parseAddItemRequest parses and validates the request to add an item.
func parseAddItemRequest(r *http.Request) (*AddItemRequest, error) {
	req := &AddItemRequest{
		Name: r.FormValue("name"),
		// STEP 4-2: add a category field
		Category: r.FormValue("category"),
	}

	// STEP 4-4: add an image field
	f, _, err := r.FormFile("image")

	if err != nil {
		return nil, errors.New("Failed to read image")
	}

	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.New("Failed to get bytes data from image")
	}
	req.Image = data

	// validate the request
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	// STEP 4-2: validate the category field
	if req.Category == "" {
		return nil, errors.New("category is required")
	}
	// STEP 4-4: validate the image field
	if len(req.Image) == 0 {
		return nil, errors.New("No bytes in the image")
	}
	return req, nil
}

func (s *Handlers) parseGetItemByIDRequest(r *http.Request) (*GetItemByIDRequest, error) {
	itemID := r.PathValue("item_id")

	i, err := strconv.Atoi(itemID)

	if err != nil {
		return nil, errors.New("Item id should be int")
	}

	req := &GetItemByIDRequest{
		ItemID: i,
	}

	return req, nil
}

// AddItem is a handler to add a new item for POST /items .
func (s *Handlers) AddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, err := parseAddItemRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// STEP 4-4: uncomment on adding an implementation to store an image
	fileName, err := s.storeImage(req.Image)
	if err != nil {
		slog.Error("failed to store image: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	item := &Item{
		Name: req.Name,
		// STEP 4-2: add a category field
		Category: req.Category,
		// STEP 4-4: add an image field
		ImageName: fileName,
	}
	message := fmt.Sprintf("item received: %s", item.Name)
	slog.Info(message)

	// STEP 4-2: add an implementation to store an item
	err = s.itemRepo.Insert(ctx, item)
	if err != nil {
		slog.Error("failed to store item: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	itemJson, err := json.Marshal(item)

	if err != nil {
		slog.Error("failed to marshal item", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	message = string(itemJson)
	slog.Info(message)

	resp := AddItemResponse{
		Name:      item.Name,
		Category:  item.Category,
		ImageName: item.ImageName,
	}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetItem is a handler to return all items' name and category.
func (s *Handlers) GetItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := s.itemRepo.SelectAll(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := GetItemsResponse{Items: items}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Handlers) GetItemByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := s.parseGetItemByIDRequest(r)

	if err != nil {
		slog.Error("failed to parse get item by id request")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item, err := s.itemRepo.SelectByID(ctx, req.ItemID)

	if err != nil {
		slog.Error("failed to get item from DB")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := GetItemByIDResponse{
		Name:      item.Name,
		Category:  item.Category,
		ImageName: item.ImageName,
	}

	err = json.NewEncoder(w).Encode(resp)

	if err != nil {
		slog.Error("failed to parse json of item")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// storeImage stores an image and returns the file path and an error if any.
// this method calculates the hash sum of the image as a file name to avoid the duplication of a same file
// and stores it in the image directory.
func (s *Handlers) storeImage(image []byte) (filePath string, err error) {
	// STEP 4-4: add an implementation to store an image
	// TODO:
	// - calc hash sum
	// - build image file path
	// - check if the image already exists
	// - store image
	// - return the image file path
	hash := sha256.Sum256(image) // shift to hash value of 32 bytes
	fileName := hex.EncodeToString(hash[:]) + `.jpg`
	filePath = filepath.Join(s.imgDirPath, fileName)

	err = os.WriteFile(filePath, image, 0644)

	if err != nil {
		return "", err
	}

	return fileName, nil
}

type GetImageRequest struct {
	FileName string // path value
}

// parseGetImageRequest parses and validates the request to get an image.
func parseGetImageRequest(r *http.Request) (*GetImageRequest, error) {
	req := &GetImageRequest{
		FileName: r.PathValue("filename"), // from path parameter
	}

	// validate the request
	if req.FileName == "" {
		return nil, errors.New("filename is required")
	}

	return req, nil
}

// GetImage is a handler to return an image for GET /images/{filename} .
// If the specified image is not found, it returns the default image.
func (s *Handlers) GetImage(w http.ResponseWriter, r *http.Request) {
	req, err := parseGetImageRequest(r)
	if err != nil {
		slog.Warn("failed to parse get image request: ", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	imgPath, err := s.buildImagePath(req.FileName)
	if err != nil {
		if !errors.Is(err, errImageNotFound) {
			slog.Warn("failed to build image path: ", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// when the image is not found, it returns the default image without an error.
		slog.Debug("image not found", "filename", imgPath)
		imgPath = filepath.Join(s.imgDirPath, "default.jpg")
	}

	slog.Info("returned image", "path", imgPath)
	http.ServeFile(w, r, imgPath)
}

// buildImagePath builds the image path and validates it.
func (s *Handlers) buildImagePath(imageFileName string) (string, error) {
	imgPath := filepath.Join(s.imgDirPath, filepath.Clean(imageFileName))

	// to prevent directory traversal attacks
	rel, err := filepath.Rel(s.imgDirPath, imgPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid image path: %s", imgPath)
	}

	// validate the image suffix
	if !strings.HasSuffix(imgPath, ".jpg") && !strings.HasSuffix(imgPath, ".jpeg") {
		return "", fmt.Errorf("image path does not end with .jpg or .jpeg: %s", imgPath)
	}

	// check if the image exists
	_, err = os.Stat(imgPath)
	if err != nil {
		return imgPath, errImageNotFound
	}

	return imgPath, nil
}
