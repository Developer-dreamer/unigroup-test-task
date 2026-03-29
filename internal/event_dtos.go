package internal

import "github.com/google/uuid"

type OnProductCreated struct {
	ID          uuid.UUID
	Name        string
	Description string
	SellerID    uuid.UUID
	Price       int64
	Amount      int
}

type OnProductDeleted struct {
	ID uuid.UUID
}
