-- +goose Up
CREATE TABLE thing (
    id    text PRIMARY KEY,
    name  text NOT NULL,
    seq   integer NOT NULL
);

-- +goose Down
DROP TABLE thing;
