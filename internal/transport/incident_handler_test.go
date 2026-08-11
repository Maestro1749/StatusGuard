package transport_test

import (
	"StatusGuard/internal/incident"
	"StatusGuard/internal/transport"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

type MockIncidentService struct {
	mock.Mock
}

func (m *MockIncidentService) GetOpen(ctx context.Context) ([]incident.Incident, error) {
	args := m.Called(ctx)
	return args.Get(0).([]incident.Incident), args.Error(1)
}

func (m *MockIncidentService) GetAllOpenByTargetID(ctx context.Context, targetID int) ([]incident.Incident, error) {
	args := m.Called(ctx, targetID)
	return args.Get(0).([]incident.Incident), args.Error(1)
}

func TestIncidentHandler_GetOpen(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(m *MockIncidentService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Успшено получен список",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetOpen", mock.Anything).Return(
					[]incident.Incident{
						{
							ID:       1,
							TargetID: 1,
						},
						{
							ID:       2,
							TargetID: 2,
						},
					}, nil,
				).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody: `[
				{
					"id":1,
					"target_id":1,
					"checks_failed":0,
					"last_error":null,
					"resolved_at":null,
					"started_at":"0001-01-01T00:00:00Z",
					"status":""
				},
				{
					"id":2,
					"target_id":2,
					"checks_failed":0,
					"last_error":null,
					"resolved_at":null,
					"started_at":"0001-01-01T00:00:00Z",
					"status":""
				}
			]`,
		},
		{
			name: "Открытые инциденты не найдены",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetOpen", mock.Anything).Return(([]incident.Incident)(nil), incident.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   incident.ErrNotFound.Error(),
		},
		{
			name: "Таймаут БД",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetOpen", mock.Anything).Return(([]incident.Incident)(nil), incident.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   incident.ErrTimeout.Error(),
		},
		{
			name: "Неизвестная ошибка сервера",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetOpen", mock.Anything).Return(([]incident.Incident)(nil), incident.ErrInternalServer).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   incident.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockIncidentService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewIncidentHandler(mockService, logger)

			r := chi.NewRouter()

			r.Get("/incidents/open", handler.GetOpen)

			req := httptest.NewRequest(http.MethodGet, "/incidents/open", nil)
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

func TestIncidentHandler_GetAllOpenByTargetID(t *testing.T) {
	tests := []struct {
		name           string
		targetID       string
		mockSetup      func(m *MockIncidentService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:     "Успшено получен список",
			targetID: "1",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetAllOpenByTargetID", mock.Anything, 1).Return([]incident.Incident{
					{
						ID:       1,
						TargetID: 1,
					},
					{
						ID:       2,
						TargetID: 1,
					},
				}, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody: `[
				{
					"id":1,
					"target_id":1,
					"checks_failed":0,
					"last_error":null,
					"resolved_at":null,
					"started_at":"0001-01-01T00:00:00Z",
					"status":""
				},
				{
					"id":2,
					"target_id":1,
					"checks_failed":0,
					"last_error":null,
					"resolved_at":null,
					"started_at":"0001-01-01T00:00:00Z",
					"status":""
				}
			]`,
		},
		{
			name:           "Невалидный id",
			targetID:       "abc",
			mockSetup:      func(m *MockIncidentService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid target id",
		},
		{
			name:     "Таймаут БД",
			targetID: "1",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetAllOpenByTargetID", mock.Anything, 1).Return(([]incident.Incident)(nil), incident.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   incident.ErrTimeout.Error(),
		},
		{
			name:     "Открытые инциденты не найдены",
			targetID: "1",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetAllOpenByTargetID", mock.Anything, 1).Return(([]incident.Incident)(nil), incident.ErrNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   incident.ErrNotFound.Error(),
		},
		{
			name:     "Неизвестная ошибка сервера",
			targetID: "1",
			mockSetup: func(m *MockIncidentService) {
				m.On("GetAllOpenByTargetID", mock.Anything, 1).Return(([]incident.Incident)(nil), incident.ErrInternalServer).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   incident.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockIncidentService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewIncidentHandler(mockService, logger)

			r := chi.NewRouter()

			r.Get("/targets/{id}/incidents", handler.GetAllOpenByTargetID)

			req := httptest.NewRequest(http.MethodGet, "/targets/"+tt.targetID+"/incidents", nil)
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
