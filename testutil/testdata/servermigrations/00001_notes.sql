-- +goose Up
-- Portable across every first-class engine: no dialect-specific type or clause.
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

-- +goose Down
DROP TABLE notes;
