-- +goose Up
-- +goose StatementBegin
CREATE TABLE products
(
    id          UUID PRIMARY KEY,
    name        VARCHAR(256) NOT NULL,
    description TEXT NULL,
    seller_id   UUID,
    price       BIGINT NOT NULL,
    amount      INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE products;
-- +goose StatementEnd
