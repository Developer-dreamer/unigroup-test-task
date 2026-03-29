package product

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"unigroup-test-task/internal"
)

type CreateRequest struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Seller      uuid.UUID `json:"seller"`
	Price       int64     `json:"price"`
}

type ProductResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Seller      uuid.UUID `json:"seller"`
	Price       int64     `json:"price"`
}

type ProductService interface {
	PostProduct(ctx context.Context, p Product) error
	DeleteProduct(ctx context.Context, id uuid.UUID) error
	GetProducts(ctx context.Context, limit, offset int) ([]Product, error)
}

type Handler struct {
	logger  internal.Logger
	service ProductService
}

func NewHandler(l internal.Logger, s ProductService) (*Handler, error) {
	if l == nil {
		return nil, internal.ErrNilLogger
	}
	if s == nil {
		return nil, errors.New("product service is nil")
	}

	return &Handler{
		logger:  l,
		service: s,
	}, nil
}

func (h *Handler) PostProduct(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userProduct := &CreateRequest{}
	err := internal.FromJSON(r.Body, userProduct)
	if err != nil {
		internal.WriteJSONError(rw, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := userProduct.Validate(); err != nil {
		internal.WriteJSONError(rw, http.StatusBadRequest, "validation failed", err)
		return
	}

	domainProduct := userProduct.RequestToDomain()
	err = h.service.PostProduct(ctx, domainProduct)
	if err != nil {
		internal.WriteJSONError(rw, http.StatusInternalServerError, "failed to post product", err)
		return
	}

	response := ToResponse(domainProduct)
	internal.WriteJSONResponse(rw, http.StatusCreated, response)
}

func (h *Handler) GetProducts(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10 // Default limit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0 // Default offset
	}

	products, err := h.service.GetProducts(ctx, limit, offset)
	if err != nil {
		internal.WriteJSONError(rw, http.StatusInternalServerError, "failed to get products", err)
		return
	}

	response := make([]ProductResponse, 0, len(products))
	for _, p := range products {
		response = append(response, ToResponse(p))
	}

	internal.WriteJSONResponse(rw, http.StatusOK, response)
}

func (h *Handler) DeleteProduct(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	idStr := vars["id"]

	productID, err := uuid.Parse(idStr)
	if err != nil {
		internal.WriteJSONError(rw, http.StatusBadRequest, "invalid product id format", err)
		return
	}

	err = h.service.DeleteProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			internal.WriteJSONError(rw, http.StatusNotFound, "product not found", err)
			return
		}
		internal.WriteJSONError(rw, http.StatusInternalServerError, "failed to delete product", err)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (r *CreateRequest) RequestToDomain() Product {
	return Product{
		ID:          uuid.New(),
		Name:        r.Name,
		Description: r.Description,
		SellerID:    r.Seller,
		Price:       r.Price,
	}
}

func ToResponse(domain Product) ProductResponse {
	return ProductResponse{
		ID:          domain.ID,
		Name:        domain.Name,
		Description: domain.Description,
		Seller:      domain.SellerID,
		Price:       domain.Price,
	}
}

func (r *CreateRequest) Validate() error {
	if r.Name == "" {
		return errors.New("product name is required")
	}
	if r.Price < 0 {
		return errors.New("price cannot be negative")
	}
	if r.Seller == uuid.Nil {
		return errors.New("seller ID is required")
	}
	return nil
}
