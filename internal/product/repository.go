package product

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"time"
	"unigroup-test-task/internal"
)

type ProductDB struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	SellerID    uuid.UUID `db:"seller_id"`
	Price       int64     `db:"price"`
	Amount      int       `db:"amount"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	DeletedAt   time.Time `db:"deleted_at"`
}

var ErrProductNotFound = errors.New("product not found")

type Repository struct {
	logger internal.Logger
	db     *sqlx.DB
}

func NewRepository(l internal.Logger, db *sqlx.DB) (*Repository, error) {
	if l == nil {
		return nil, internal.ErrNilLogger
	}
	if db == nil {
		return nil, errors.New("db is nil")
	}

	return &Repository{
		logger: l,
		db:     db,
	}, nil
}

func (r *Repository) GetProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	query := `
		SELECT id, name, description, seller_id, price, amount
		FROM products 
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	r.logger.InfoContext(ctx, "executing query to get products", "query", query, "limit", limit, "offset", offset, "repository", "Repository")

	var dbProducts []ProductDB

	err := r.db.SelectContext(ctx, &dbProducts, query, limit, offset)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to get products", "error", err)
		return nil, err
	}

	products := make([]Product, 0, len(dbProducts))
	for _, dbP := range dbProducts {
		products = append(products, dbP.ToDomain())
	}

	return products, nil
}

func (r *Repository) InsertProduct(ctx context.Context, product Product) error {
	dbProduct := FromDomain(product)
	dbProduct.CreatedAt = time.Now().UTC()
	dbProduct.UpdatedAt = dbProduct.CreatedAt

	query := `
		INSERT INTO products (id, name, description, seller_id, price, amount, created_at, updated_at)
		VALUES (:id, :name, :description, :seller_id, :price, :amount, :created_at, :updated_at)
	`

	r.logger.InfoContext(ctx, "executing query to insert new product", "query", query, "repository", "Repository")

	var ext interface {
		NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	} = r.db

	if tx, ok := ctx.Value("tx").(*sqlx.Tx); ok {
		ext = tx
	}

	_, err := ext.NamedExecContext(ctx, query, dbProduct)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to insert new product", "error", err)
		return err
	}

	return nil
}

func (r *Repository) DeleteProductByID(ctx context.Context, id uuid.UUID) error {
	deletedAt := time.Now().UTC()

	query := `
		UPDATE products
		SET deleted_at = $2
		WHERE id = $1 and deleted_at is null
	`

	r.logger.InfoContext(ctx, "executing query to delete product", "query", query, "repository", "Repository")

	var ext interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	} = r.db

	if tx, ok := ctx.Value("tx").(*sqlx.Tx); ok {
		ext = tx
	}

	res, err := ext.ExecContext(ctx, query, id, deletedAt)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to delete product", "error", err)
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to delete product", "error", err)
		return err
	}
	if rowsAffected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func FromDomain(d Product) ProductDB {
	return ProductDB{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		SellerID:    d.SellerID,
		Price:       d.Price,
		Amount:      d.Amount,
	}
}

func (p *ProductDB) ToDomain() Product {
	return Product{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		SellerID:    p.SellerID,
		Price:       p.Price,
		Amount:      p.Amount,
	}
}
