package product

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"unigroup-test-task/internal/event"
)

type mockProductRepo struct {
	mock.Mock
}

func (m *mockProductRepo) InsertProduct(ctx context.Context, p Product) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockProductRepo) DeleteProductByID(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockProductRepo) GetProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Product), args.Error(1)
}

type mockEventRepo struct {
	mock.Mock
}

func (m *mockEventRepo) CreateEvent(ctx context.Context, e event.Event) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

// fakeTransactor executes function without real transaction
type fakeTransactor struct{}

func (f *fakeTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestService_GetProducts(t *testing.T) {
	productRepo := new(mockProductRepo)
	eventRepo := new(mockEventRepo)
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc, _ := NewService(nopLogger, productRepo, eventRepo, &fakeTransactor{})

	ctx := context.Background()
	expectedProducts := []Product{
		{ID: uuid.New(), Name: "Test 1", Price: 100},
		{ID: uuid.New(), Name: "Test 2", Price: 200},
	}

	productRepo.On("GetProducts", ctx, 10, 0).Return(expectedProducts, nil)

	products, err := svc.GetProducts(ctx, 10, 0)

	require.NoError(t, err)
	require.Len(t, products, 2)
	require.Equal(t, expectedProducts, products)
	productRepo.AssertExpectations(t)
}

func TestService_PostProduct(t *testing.T) {
	productRepo := new(mockProductRepo)
	eventRepo := new(mockEventRepo)
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc, _ := NewService(nopLogger, productRepo, eventRepo, &fakeTransactor{})

	ctx := context.Background()
	newProduct := Product{
		ID:    uuid.New(),
		Name:  "New Product",
		Price: 500,
	}

	productRepo.On("InsertProduct", ctx, newProduct).Return(nil)

	eventRepo.On("CreateEvent", ctx, mock.AnythingOfType("event.Event")).Return(nil)

	err := svc.PostProduct(ctx, newProduct)

	require.NoError(t, err)
	productRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestService_DeleteProduct(t *testing.T) {
	productRepo := new(mockProductRepo)
	eventRepo := new(mockEventRepo)
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc, _ := NewService(nopLogger, productRepo, eventRepo, &fakeTransactor{})

	ctx := context.Background()
	productID := uuid.New()

	productRepo.On("DeleteProductByID", ctx, productID).Return(nil)
	eventRepo.On("CreateEvent", ctx, mock.AnythingOfType("event.Event")).Return(nil)

	err := svc.DeleteProduct(ctx, productID)

	require.NoError(t, err)
	productRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestService_DeleteProduct_NotFound(t *testing.T) {
	productRepo := new(mockProductRepo)
	eventRepo := new(mockEventRepo)
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc, _ := NewService(nopLogger, productRepo, eventRepo, &fakeTransactor{})

	ctx := context.Background()
	productID := uuid.New()

	productRepo.On("DeleteProductByID", ctx, productID).Return(ErrProductNotFound)

	err := svc.DeleteProduct(ctx, productID)

	require.ErrorIs(t, err, ErrProductNotFound)

	eventRepo.AssertNotCalled(t, "CreateEvent")
}
