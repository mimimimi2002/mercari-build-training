DROP TABLE IF EXISTS items;

CREATE TABLE items (
  id INTEGER PRIMARY KEY,
  name TEXT,
  category_id INTEGER,
  image_name TEXT,
  FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE TABLE categories (
  id INTEGER PRIMARY KEY,
  name TEXT
);