package transport_test

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/monitor"
	"StatusGuard/internal/transport"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

type MockCheckerService struct {
	mock.Mock
}

func (m *MockCheckerService) CheckManually(ctx context.Context, id int) (*checker.Result, *time.Duration, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*checker.Result), args.Get(1).(*time.Duration), args.Error(2)
}

func (m *MockCheckerService) GetCheckHistory(ctx context.Context, targetID int, limit int) ([]checker.Result, error) {
	args := m.Called(ctx, targetID, limit)
	return args.Get(0).([]checker.Result), args.Error(1)
}

func TestCheckerHandler_CheckTarget(t *testing.T) {
	validResult := checker.Result{ID: 1, TargetID: 1}

	tests := []struct {
		name           string
		targetID       string
		mockSetup      func(m *MockCheckerService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:     "Успешная ручная проверка",
			targetID: "1",
			mockSetup: func(m *MockCheckerService) {
				m.On("CheckManually", mock.Anything, 1).Return(&validResult, (*time.Duration)(nil), nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"id":1,"target_id":1`,
		},
		{
			name:           "Невалидный id",
			targetID:       "abc",
			mockSetup:      func(m *MockCheckerService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid target id",
		},
		{
			name:     "Сущность не найдена",
			targetID: "99",
			mockSetup: func(m *MockCheckerService) {
				m.On("CheckManually", mock.Anything, 99).Return((*checker.Result)(nil), (*time.Duration)(nil), monitor.ErrTargetNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   monitor.ErrTargetNotFound.Error(),
		},
		{
			name:     "Слишком много запросов",
			targetID: "1",
			mockSetup: func(m *MockCheckerService) {
				m.On("CheckManually", mock.Anything, 1).Return((*checker.Result)(nil), (*time.Duration)(nil), checker.ErrTooManyRequests).Once()
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   checker.ErrTooManyRequests.Error(),
		},
		{
			name:     "Таймаут БД",
			targetID: "1",
			mockSetup: func(m *MockCheckerService) {
				m.On("CheckManually", mock.Anything, 1).Return((*checker.Result)(nil), (*time.Duration)(nil), checker.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   checker.ErrTimeout.Error(),
		},
		{
			name:     "Неизвестная ошибка сервера",
			targetID: "1",
			mockSetup: func(m *MockCheckerService) {
				m.On("CheckManually", mock.Anything, 1).Return((*checker.Result)(nil), (*time.Duration)(nil), checker.ErrInternalServer).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   checker.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockCheckerService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewCheckerHandler(mockService, logger)

			r := chi.NewRouter()

			r.Post("/targets/{id}/check", handler.CheckTarget)

			req := httptest.NewRequest(http.MethodPost, "/targets/"+tt.targetID+"/check", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "HTTP статус не совпадает")

			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody, "Тело ответа не совпадает")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestCheckerHandler_GetCheckHistory(t *testing.T) {
	tests := []struct {
		name           string
		targetID       string
		limit          string
		mockSetup      func(m *MockCheckerService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:     "Успешно получен список",
			targetID: "1",
			limit:    "2",
			mockSetup: func(m *MockCheckerService) {
				m.On("GetCheckHistory", mock.Anything, 1, 2).Return(
					[]checker.Result{
						{
							ID:       2,
							TargetID: 1,
						},
						{
							ID:       1,
							TargetID: 1,
						},
					},
					nil,
				).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"id":2,"target_id":1,"status":"","response_time_ms":0,"http_status":null,"error_message":null,"checked_at":"0001-01-01T00:00:00Z"},{"id":1,"target_id":1,"status":"","response_time_ms":0,"http_status":null,"error_message":null,"checked_at":"0001-01-01T00:00:00Z"}]`,
		},
		{
			name:           "Невалидный id",
			targetID:       "abc",
			limit:          "5",
			mockSetup:      func(m *MockCheckerService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid target id",
		},
		{
			name:           "Невалдиный limit",
			targetID:       "1",
			limit:          "abc",
			mockSetup:      func(m *MockCheckerService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid limit",
		},
		{
			name:     "Результаты не найдены",
			targetID: "99",
			limit:    "5",
			mockSetup: func(m *MockCheckerService) {
				m.On("GetCheckHistory", mock.Anything, 99, 5).Return(([]checker.Result)(nil), checker.ErrResultsNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   checker.ErrResultsNotFound.Error(),
		},
		{
			name:     "Таймаут БД",
			targetID: "1",
			limit:    "5",
			mockSetup: func(m *MockCheckerService) {
				m.On("GetCheckHistory", mock.Anything, 1, 5).Return(([]checker.Result)(nil), checker.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   checker.ErrTimeout.Error(),
		},
		{
			name:     "Неизвестная ошибка сервера",
			targetID: "1",
			limit:    "5",
			mockSetup: func(m *MockCheckerService) {
				m.On("GetCheckHistory", mock.Anything, 1, 5).Return(([]checker.Result)(nil), checker.ErrInternalServer)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   checker.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockCheckerService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewCheckerHandler(mockService, logger)

			r := chi.NewRouter()

			r.Get("/targets/{id}/checks", handler.GetCheckHistory)

			req := httptest.NewRequest(http.MethodGet, "/targets/"+tt.targetID+"/checks?limit="+tt.limit, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "HTTP статус не совпадает")

			if tt.expectedBody != "" {
				if strings.HasPrefix(tt.expectedBody, "{") || strings.HasPrefix(tt.expectedBody, "[") {
					assert.JSONEq(t, tt.expectedBody, rec.Body.String(), "Тело JSON ответа не совпадает")
				} else {
					assert.Contains(t, rec.Body.String(), tt.expectedBody, "Тело ответа не совпадает")
				}
			}

			mockService.AssertExpectations(t)
		})
	}
}
