package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
)

// newMultipartItemRequest builds a POST /items request with multipart/form-data body.
// Empty name/category are skipped, and a nil image means no file part is attached.
func newMultipartItemRequest(t *testing.T, name, category string, image []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if name != "" {
		mw.WriteField("name", name)
	}
	if category != "" {
		mw.WriteField("category", category)
	}
	if image != nil {
		fw, err := mw.CreateFormFile("image", "test.jpg")
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := fw.Write(image); err != nil {
			t.Fatalf("failed to write image: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest("POST", "http://localhost:9000/items", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestParseAddItemRequest(t *testing.T) {
	t.Parallel()

	dummyImage := []byte("dummy image data")

	type args struct {
		name     string
		category string
		image    []byte
	}

	type wants struct {
		req *AddItemRequest
		err bool
	}

	// STEP 6-1: define test cases
	cases := map[string]struct {
		args args
		wants
	}{
		"ok: valid request": {
			args: args{
				name:     "test_name",
				category: "test_category",
				image:    dummyImage,
			},
			wants: wants{
				req: &AddItemRequest{
					Name:     "test_name",
					Category: "test_category",
					Image:    dummyImage,
				},
				err: false,
			},
		},
		"ng: empty request": {
			args: args{},
			wants: wants{
				req: nil,
				err: true,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// prepare HTTP request (multipart/form-data, because parseAddItemRequest reads r.FormFile)
			req := newMultipartItemRequest(t, tt.args.name, tt.args.category, tt.args.image)

			// execute test target
			got, err := parseAddItemRequest(req)

			// confirm the result
			if err != nil {
				if !tt.err {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if diff := cmp.Diff(tt.wants.req, got); diff != "" {
				t.Errorf("unexpected request (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHelloHandler(t *testing.T) {
	t.Parallel()

	// Please comment out for STEP 6-2
	// predefine what we want
	type wants struct {
		code int               // desired HTTP status code
		body map[string]string // desired body
	}
	want := wants{
		code: http.StatusOK,
		body: map[string]string{"message": "Hello, world!"},
	}

	// set up test
	req := httptest.NewRequest("GET", "/hello", nil)
	res := httptest.NewRecorder()

	h := &Handlers{}
	h.Hello(res, req)

	// STEP 6-2: confirm the status code
	if res.Code != want.code {
		t.Errorf("status code is not StatusOK")
	}

	// STEP 6-2: confirm response body
	var got map[string]string
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if diff := cmp.Diff(want.body, got); diff != "" {
		t.Errorf("unexpected response body (-want +got):\n%s", diff)
	}
}

func TestAddItem(t *testing.T) {
	t.Parallel()

	dummyImage := []byte("dummy image")

	type args struct {
		name     string
		category string
		image    []byte
	}

	type wants struct {
		code int
	}

	cases := map[string]struct {
		args     args
		injector func(m *MockItemRepository)
		wants
	}{
		"ok: correctly inserted": {
			args: args{
				name:     "used iPhone 16e",
				category: "phone",
				image:    dummyImage,
			},
			injector: func(m *MockItemRepository) {
				m.EXPECT().
					Insert(gomock.Any(), gomock.Any()).
					Return(nil)
				// STEP 6-3: define mock expectation
				// succeeded to insert
			},
			wants: wants{
				code: http.StatusOK,
			},
		},
		"ng: failed to insert": {
			args: args{
				name:     "used iPhone 16e",
				category: "phone",
				image:    dummyImage,
			},
			injector: func(m *MockItemRepository) {
				m.EXPECT().
					Insert(gomock.Any(), gomock.Any()).
					Return(errors.New("db error"))
				// STEP 6-3: define mock expectation
				// failed to insert
			},
			wants: wants{
				code: http.StatusInternalServerError,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			mockIR := NewMockItemRepository(ctrl)
			tt.injector(mockIR)
			// imgDirPath points to a temp dir so storeImage does not pollute the repo.
			h := &Handlers{itemRepo: mockIR, imgDirPath: t.TempDir()}

			req := newMultipartItemRequest(t, tt.args.name, tt.args.category, tt.args.image)

			rr := httptest.NewRecorder()
			h.AddItem(rr, req)

			if tt.wants.code != rr.Code {
				t.Errorf("expected status code %d, got %d", tt.wants.code, rr.Code)
			}
			if tt.wants.code >= 400 {
				return
			}

			for _, v := range []string{tt.args.name, tt.args.category} {
				if !strings.Contains(rr.Body.String(), v) {
					t.Errorf("response body does not contain %s, got: %s", v, rr.Body.String())
				}
			}
		})
	}
}

// STEP 6-4: uncomment this test
func TestAddItemE2e(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	db, closers, err := setupDB(t)
	if err != nil {
		t.Fatalf("failed to set up database: %v", err)
	}
	t.Cleanup(func() {
		for _, c := range closers {
			c()
		}
	})

	dummyImage := []byte("Dummy image")

	type args struct {
		name     string
		category string
		image    []byte
	}
	type wants struct {
		code int
	}

	cases := map[string]struct {
		args args
		wants
	}{
		"ok: correctly inserted": {
			args: args{
				name:     "used iPhone 16e",
				category: "phone",
				image:    dummyImage,
			},
			wants: wants{
				code: http.StatusOK,
			},
		},
		"ng: missing name": {
			args: args{
				name:     "",
				category: "phone",
				image:    dummyImage,
			},
			wants: wants{
				code: http.StatusBadRequest,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			h := &Handlers{itemRepo: &itemRepository{db: db}, imgDirPath: t.TempDir()}

			req := newMultipartItemRequest(t, tt.args.name, tt.args.category, tt.args.image)

			rr := httptest.NewRecorder()
			h.AddItem(rr, req)

			// check response
			if tt.wants.code != rr.Code {
				t.Errorf("expected status code %d, got %d", tt.wants.code, rr.Code)
			}
			if tt.wants.code >= 400 {
				return
			}
			for _, v := range []string{tt.args.name, tt.args.category} {
				if !strings.Contains(rr.Body.String(), v) {
					t.Errorf("response body does not contain %s, got: %s", v, rr.Body.String())
				}
			}

			// STEP 6-4: check inserted data
			// Query the real test DB and confirm the item was stored as expected.
			var gotName, gotCategory string
			query := "SELECT i.name, c.name FROM items AS i " +
				"JOIN categories AS c ON i.category_id = c.id WHERE i.name = ?"
			if err := db.QueryRow(query, tt.args.name).Scan(&gotName, &gotCategory); err != nil {
				t.Fatalf("failed to query inserted item: %v", err)
			}
			if gotName != tt.args.name {
				t.Errorf("inserted name mismatch: want %q, got %q", tt.args.name, gotName)
			}
			if gotCategory != tt.args.category {
				t.Errorf("inserted category mismatch: want %q, got %q", tt.args.category, gotCategory)
			}
		})
	}
}

func setupDB(t *testing.T) (db *sql.DB, closers []func(), e error) {
	t.Helper()

	defer func() {
		if e != nil {
			for _, c := range closers {
				c()
			}
		}
	}()

	// create a temporary file for e2e testing
	f, err := os.CreateTemp(".", "*.sqlite3")
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, func() {
		f.Close()
		os.Remove(f.Name())
	})

	// set up tables
	db, err = sql.Open("sqlite3", f.Name())
	if err != nil {
		return nil, nil, err
	}
	closers = append(closers, func() {
		db.Close()
	})

	cmd := `CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY,
		name TEXT
	);
	CREATE TABLE IF NOT EXISTS items (
		id INTEGER PRIMARY KEY,
		name TEXT,
		category_id INTEGER,
		image_name TEXT,
		FOREIGN KEY (category_id) REFERENCES categories(id)
	);`
	_, err = db.Exec(cmd)
	if err != nil {
		return nil, nil, err
	}

	return db, closers, nil
}
