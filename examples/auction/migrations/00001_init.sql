-- Migration version 1.
--
-- This file is empty on purpose: pw creates no table for you. The example
-- below is the shape of a migration, written for this project's engine.
-- Uncomment it, or replace it with your own schema, and pw dev applies it.
--
-- Sample rows do not belong here. A migration runs in every environment,
-- including production, and an applied one cannot be edited. Use pw seed and
-- a dataset for development data.

-- +goose Up
-- CREATE TABLE example (
--     id INTEGER PRIMARY KEY,
--     name TEXT NOT NULL
-- );

-- +goose Down
-- DROP TABLE example;
