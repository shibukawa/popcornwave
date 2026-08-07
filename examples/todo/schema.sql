-- The one table both implementations read and write. Applied by compare.sh
-- before either service starts, so neither owns the schema.
CREATE TABLE IF NOT EXISTS todos (
    id         BIGSERIAL   PRIMARY KEY,
    title      TEXT        NOT NULL,
    done       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
