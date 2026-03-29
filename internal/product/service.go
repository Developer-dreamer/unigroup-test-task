package product

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"reflect"
	"unigroup-test-task/internal"
	"unigroup-test-task/internal/event"
)

type Product struct {
	ID          uuid.UUID
	Name        string
	Description string
	SellerID    uuid.UUID
	Price       int64
	Amount      int
}

type productRepository interface {
	InsertProduct(ctx context.Context, p Product) error
	DeleteProductByID(ctx context.Context, id uuid.UUID) error
	GetProducts(ctx context.Context, limit, offset int) ([]Product, error)
}

type eventRepository interface {
	CreateEvent(ctx context.Context, event event.Event) error
}

type Service struct {
	logger      internal.Logger
	productRepo productRepository
	eventRepo   eventRepository
	transactor  event.TransactionManager
}

func NewService(l internal.Logger, pr productRepository, er eventRepository, t event.TransactionManager) (*Service, error) {
	if l == nil {
		return nil, errors.New("nil logger")
	}
	if pr == nil {
		return nil, errors.New("nil product repository")
	}
	if er == nil {
		return nil, errors.New("nil event repository")
	}
	if t == nil {
		return nil, errors.New("nil transaction manager")
	}

	return &Service{
		logger:      l,
		productRepo: pr,
		eventRepo:   er,
		transactor:  t,
	}, nil
}

func (s *Service) PostProduct(ctx context.Context, p Product) error {
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.productRepo.InsertProduct(txCtx, p); err != nil {
			return err
		}

		productCreated := internal.OnProductCreated{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			SellerID:    p.SellerID,
			Price:       p.Price,
			Amount:      p.Amount,
		}

		productCreatedJSON, err := json.Marshal(productCreated)
		if err != nil {
			return err
		}

		evnt := event.Event{
			ID:            uuid.New(),
			AggregateType: reflect.TypeOf(p).Elem().Name(),
			AggregateID:   p.ID,
			EventType:     "post_product",
			Payload:       productCreatedJSON,
			Status:        event.Pending,
		}

		return s.eventRepo.CreateEvent(txCtx, evnt)
	})
}

func (s *Service) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.productRepo.DeleteProductByID(txCtx, id); err != nil {
			return err
		}

		productDeleted := internal.OnProductDeleted{
			ID: id,
		}

		productDeletedJSON, err := json.Marshal(productDeleted)
		if err != nil {
			return err
		}

		evnt := event.Event{
			ID:            uuid.New(),
			AggregateType: reflect.TypeOf(id).Name(),
			AggregateID:   id,
			EventType:     "delete_product",
			Payload:       productDeletedJSON,
			Status:        event.Pending,
		}

		return s.eventRepo.CreateEvent(txCtx, evnt)
	})
}

func (s *Service) GetProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	return s.productRepo.GetProducts(ctx, limit, offset)
}
