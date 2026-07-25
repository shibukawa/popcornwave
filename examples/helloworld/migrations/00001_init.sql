-- +goose Up
CREATE TABLE access_counter (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    count INTEGER NOT NULL
);

-- +goose Down
DROP TABLE access_counter;
