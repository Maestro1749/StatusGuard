package transport_test

import (
	"StatusGuard/internal/monitor"
	"StatusGuard/internal/transport"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

type MockTargetsService struct {
	mock.Mock
}

func (m *MockTargetsService) CreateTarget(
	ctx context.Context,
	name string,
	urlTarget string,
	method string,
	expectedStatus int,
	intervalSeconds int,
	timeoutSeconds int,
) (*monitor.Target, error) {
	args := m.Called(ctx, name, urlTarget, method, expectedStatus, intervalSeconds, timeoutSeconds)
	return args.Get(0).(*monitor.Target), args.Error(1)
}

func (m *MockTargetsService) DeleteTarget(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTargetsService) GetTarget(ctx context.Context, id int) (*monitor.Target, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*monitor.Target), args.Error(1)
}

func (m *MockTargetsService) GetAllTargets(ctx context.Context) ([]monitor.Target, error) {
	args := m.Called(ctx)
	return args.Get(0).([]monitor.Target), args.Error(1)
}

func (m *MockTargetsService) UpdateTarget(ctx context.Context, input monitor.UpdateTargetInput) (*monitor.Target, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*monitor.Target), args.Error(1)
}

func TestTargetsHandler_CreateTarget(t *testing.T) {
	validTarget := monitor.Target{ID: 1, Name: "test-target"}

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(m *MockTargetsService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:        "Успшеное создание",
			requestBody: `{"name":"test","url":"http://test.com","method":"GET","expected_status":200,"interval_seconds":10,"timeout_seconds":5}`,
			mockSetup: func(m *MockTargetsService) {
				m.On("CreateTarget",
					mock.Anything,
					"test", "http://test.com", "GET", 200, 10, 5,
				).Return(&validTarget, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `"name":"test-target"`,
		},
		{
			name:           "Ошибка: невалидный JSON",
			requestBody:    `{"name":"test", "url":}`,
			mockSetup:      func(m *MockTargetsService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid data input",
		},
		{
			name:        "Ошибка бизнес-логики: пустое имя",
			requestBody: `{"name":"","url":"http://test.com"}`,
			mockSetup: func(m *MockTargetsService) {
				m.On("CreateTarget", mock.Anything, "", "http://test.com", "", 0, 0, 0).
					Return((*monitor.Target)(nil), monitor.ErrEmptyName).Once()
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   monitor.ErrEmptyName.Error(),
		},
		{
			name:        "Ошибка инфраструктуры: таймаут создания",
			requestBody: `{"name":"test","url":"http://test.com"}`,
			mockSetup: func(m *MockTargetsService) {
				m.On("CreateTarget", mock.Anything, "test", "http://test.com", "", 0, 0, 0).
					Return((*monitor.Target)(nil), monitor.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   monitor.ErrTimeout.Error(),
		},
		{
			name:        "Неизвестная ошибка: Internal server error",
			requestBody: `{"name":"test","url":"http://test.com"}`,
			mockSetup: func(m *MockTargetsService) {
				m.On("CreateTarget", mock.Anything, "test", "http://test.com", "", 0, 0, 0).
					Return((*monitor.Target)(nil), errors.New("unexpected database error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   monitor.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTargetsService)
			tt.mockSetup(mockService)

			logger := zaptest.NewLogger(t)

			handler := transport.NewMonitorHandler(logger, mockService)

			req := httptest.NewRequest(http.MethodPost, "/targets", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.CreateTarget(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "HTTP статус не совпадает")

			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody, "Тело ответа не содержит ожидаемых данных")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestTargetsHandler_DeleteTarget(t *testing.T) {
	tests := []struct {
		name           string
		targetID       string
		mockSetup      func(m *MockTargetsService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:     "Упшеное удаление",
			targetID: "1",
			mockSetup: func(m *MockTargetsService) {
				m.On("DeleteTarget", mock.Anything, 1).Return(nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:           "Ошибка парсинга ID",
			targetID:       "abc",
			mockSetup:      func(m *MockTargetsService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid target id",
		},
		{
			name:     "Сущность не найдена",
			targetID: "99",
			mockSetup: func(m *MockTargetsService) {
				m.On("DeleteTarget", mock.Anything, 99).Return(monitor.ErrTargetNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   monitor.ErrTargetNotFound.Error(),
		},
		{
			name:     "Таймаут БД",
			targetID: "1",
			mockSetup: func(m *MockTargetsService) {
				m.On("DeleteTarget", mock.Anything, 1).Return(monitor.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   monitor.ErrTimeout.Error(),
		},
		{
			name:     "Неизвестная ошибка сервера",
			targetID: "1",
			mockSetup: func(m *MockTargetsService) {
				m.On("DeleteTarget", mock.Anything, 1).Return(monitor.ErrInternalServer).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   monitor.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTargetsService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewMonitorHandler(logger, mockService)

			r := chi.NewRouter()

			r.Delete("/targets/{id}", handler.DeleteTarget)

			req := httptest.NewRequest(http.MethodDelete, "/targets/"+tt.targetID, nil)
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

func TestTargetsHandler_GetTarget(t *testing.T) {
	validTarget := monitor.Target{ID: 1, Name: "test-target"}

	tests := []struct {
		name           string
		targetID       string
		mockSetup      func(m *MockTargetsService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:     "Успшено получен target",
			targetID: "1",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetTarget", mock.Anything, 1).Return(&validTarget, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"name":"test-target"`,
		},
		{
			name:           "Ошибка парсинга ID",
			targetID:       "abc",
			mockSetup:      func(m *MockTargetsService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid target id",
		},
		{
			name:     "Сущность не найдена",
			targetID: "99",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetTarget", mock.Anything, 99).Return((*monitor.Target)(nil), monitor.ErrTargetNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   monitor.ErrTargetNotFound.Error(),
		},
		{
			name:     "Таймаут БД",
			targetID: "1",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetTarget", mock.Anything, 1).Return((*monitor.Target)(nil), monitor.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   monitor.ErrTimeout.Error(),
		},
		{
			name:     "Неизвестная ошибка сервера",
			targetID: "1",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetTarget", mock.Anything, 1).Return((*monitor.Target)(nil), monitor.ErrInternalServer).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   monitor.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTargetsService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewMonitorHandler(logger, mockService)

			r := chi.NewRouter()

			r.Get("/targets/{id}", handler.GetTarget)

			req := httptest.NewRequest(http.MethodGet, "/targets/"+tt.targetID, nil)
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

func TestTargetsHandler_GetAllTargets(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(m *MockTargetsService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Успешно получен список",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetAllTargets", mock.Anything).Return([]monitor.Target{
					{
						ID:   1,
						Name: "test1",
					},
					{
						ID:   2,
						Name: "test2",
					},
				},
					nil,
				).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"name":"test1"`,
		},
		{
			name: "Таймаут БД",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetAllTargets", mock.Anything).Return(([]monitor.Target)(nil), monitor.ErrTimeout).Once()
			},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   monitor.ErrTimeout.Error(),
		},
		{
			name: "Неизвестная ошибка сервера",
			mockSetup: func(m *MockTargetsService) {
				m.On("GetAllTargets", mock.Anything).Return(([]monitor.Target)(nil), monitor.ErrInternalServer).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   monitor.ErrInternalServer.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTargetsService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewMonitorHandler(logger, mockService)

			req := httptest.NewRequest(http.MethodGet, "/targets", nil)
			rec := httptest.NewRecorder()

			handler.GetAllTargets(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "HTTP статус не совпадает")

			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody, "Тело ответа не совпадает")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }
func ptrBool(b bool) *bool    { return &b }

func TestTargetHandler_UpdateTarget(t *testing.T) {
	validTarget := monitor.Target{ID: 1, Name: "updatedTarget"}

	tests := []struct {
		name           string
		targetID       string
		requestBody    string
		mockSetup      func(m *MockTargetsService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:        "Полное обновление",
			targetID:    "1",
			requestBody: `{"name":"updated","url":"http://test.com","method":"GET","expected_status":200,"interval_seconds":10,"timeout_seconds":5,"enabled":true}`,
			mockSetup: func(m *MockTargetsService) {
				expectedInput := monitor.UpdateTargetInput{
					ID:              1,
					Name:            ptrStr("updated"),
					URL:             ptrStr("http://test.com"),
					Method:          ptrStr("GET"),
					ExpectedStatus:  ptrInt(200),
					IntervalSeconds: ptrInt(10),
					TimeoutSeconds:  ptrInt(5),
					Enabled:         ptrBool(true),
				}
				m.On("UpdateTarget", mock.Anything, expectedInput).Return(&validTarget, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"name":"updatedTarget"`,
		},
		{
			name:        "Частичное обновление: только имя",
			targetID:    "1",
			requestBody: `{"name":"updated"}`,
			mockSetup: func(m *MockTargetsService) {
				expectedInput := monitor.UpdateTargetInput{
					ID:   1,
					Name: ptrStr("updated"),
				}
				m.On("UpdateTarget", mock.Anything, expectedInput).Return(&validTarget, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"name":"updatedTarget"`,
		},
		{
			name:        "Частичное обновление: только URL",
			targetID:    "1",
			requestBody: `{"url":"http://newtest.com"}`,
			mockSetup: func(m *MockTargetsService) {
				expectedInput := monitor.UpdateTargetInput{
					ID:  1,
					URL: ptrStr("http://newtest.com"),
				}
				m.On("UpdateTarget", mock.Anything, expectedInput).Return(&validTarget, nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"name":"updatedTarget"`,
		},
		{
			name:           "Ошибка парсинга id",
			targetID:       "abc",
			requestBody:    `{"name":"newName"}`,
			mockSetup:      func(m *MockTargetsService) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid target id",
		},
		{
			name:        "Невалидный URL",
			targetID:    "1",
			requestBody: `{"url":"ht://testcom"}`,
			mockSetup: func(m *MockTargetsService) {
				expectedInput := monitor.UpdateTargetInput{
					ID:  1,
					URL: ptrStr("ht://testcom"),
				}
				m.On("UpdateTarget", mock.Anything, expectedInput).Return((*monitor.Target)(nil), monitor.ErrInvalidURL).Once()
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   monitor.ErrInvalidURL.Error(),
		},
		{
			name:           "Битый JSON",
			targetID:       "1",
			requestBody:    `{"name":}`,
			mockSetup:      func(m *MockTargetsService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTargetsService)
			tt.mockSetup(mockService)
			logger := zaptest.NewLogger(t)

			handler := transport.NewMonitorHandler(logger, mockService)

			r := chi.NewRouter()

			r.Patch("/targets/{id}", handler.UpdateTarget)

			req := httptest.NewRequest(http.MethodPatch, "/targets/"+tt.targetID, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
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
