package checker_test

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/monitor"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type MockTargetProvider struct {
	GetByIDFunc func(ctx context.Context, id int) (*monitor.Target, error)
}

type MockCheckerRepository struct {
	SaveCallCount     int
	SaveFunc          func(ctx context.Context, result checker.Result) (*checker.Result, error)
	GetByTargetIDFunc func(ctx context.Context, targetID int, limit int) ([]checker.Result, error)
}

type MockrateLimiter struct {
	AllowFunc func(ctx context.Context, key string) (bool, time.Duration, error)
}

type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (t *MockTargetProvider) GetByID(ctx context.Context, id int) (*monitor.Target, error) {
	return t.GetByIDFunc(ctx, id)
}

func (c *MockCheckerRepository) Save(ctx context.Context, result checker.Result) (*checker.Result, error) {
	c.SaveCallCount++
	if c.SaveFunc != nil {
		return c.SaveFunc(ctx, result)
	}
	return &result, nil
}

func (c *MockCheckerRepository) GetByTargetID(ctx context.Context, targetID int, limit int) ([]checker.Result, error) {
	return c.GetByTargetIDFunc(ctx, targetID, limit)
}

func (r *MockrateLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	return r.AllowFunc(ctx, key)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}

	return &http.Response{
		StatusCode: http.StatusInternalServerError,
	}, nil
}

func TestCheckerService_CheckManually(t *testing.T) {
	type testCase struct {
		name                          string
		inputId                       int
		mockTargetProviderBehavior    func(t *MockTargetProvider)
		mockRateLimiterBehavior       func(r *MockrateLimiter)
		mockCheckerRepositoryBehavior func(c *MockCheckerRepository)
		mockHTTPBehavior              func(m *MockRoundTripper)
		wantResult                    checker.Result
		wantErr                       error
	}

	var errMockDB = errors.New("simulated db failure")
	var errMockRedis = errors.New("simulated redis timeout")
	var errResponseTimeout = errors.New("dial tcp: i/o timeout")

	tests := []testCase{
		{
			name:    "Позитивный: Успешная ручная проверка",
			inputId: 1,
			mockTargetProviderBehavior: func(t *MockTargetProvider) {
				t.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1}, nil
				}
			},
			mockRateLimiterBehavior: func(r *MockrateLimiter) {
				r.AllowFunc = func(ctx context.Context, key string) (bool, time.Duration, error) {
					return true, 0, nil
				}
			},
			mockCheckerRepositoryBehavior: func(c *MockCheckerRepository) {
				c.SaveFunc = func(ctx context.Context, result checker.Result) (*checker.Result, error) {
					return &checker.Result{ID: 1, TargetID: 1, Status: checker.StatusUp}, nil
				}
			},
			mockHTTPBehavior: func(m *MockRoundTripper) {
				m.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
					}, nil
				}
			},
			wantResult: checker.Result{
				ID:       1,
				TargetID: 1,
				Status:   checker.StatusUp,
			},
			wantErr: nil,
		},
		{
			name:    "Негативный: Не найден target",
			inputId: 22,
			mockTargetProviderBehavior: func(t *MockTargetProvider) {
				t.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return nil, monitor.ErrTargetNotFound
				}
			},
			wantErr: monitor.ErrTargetNotFound,
		},
		{
			name:    "Негативный: Слишком частые запросы",
			inputId: 1,
			mockTargetProviderBehavior: func(t *MockTargetProvider) {
				t.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1}, nil
				}
			},
			mockRateLimiterBehavior: func(r *MockrateLimiter) {
				r.AllowFunc = func(ctx context.Context, key string) (bool, time.Duration, error) {
					return false, 5 * time.Minute, nil
				}
			},
			wantErr: checker.ErrTooManyRequests,
		},
		{
			name:    "Негативный: Ошибка инфраструктуры rate limiter",
			inputId: 1,
			mockTargetProviderBehavior: func(t *MockTargetProvider) {
				t.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1}, nil
				}
			},
			mockRateLimiterBehavior: func(r *MockrateLimiter) {
				r.AllowFunc = func(ctx context.Context, key string) (bool, time.Duration, error) {
					return false, 0, errMockRedis
				}
			},
			wantErr: errMockRedis,
		},
		{
			name:    "Негативный: Ошибка сохранения результата в БД",
			inputId: 1,
			mockTargetProviderBehavior: func(t *MockTargetProvider) {
				t.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1}, nil
				}
			},
			mockRateLimiterBehavior: func(r *MockrateLimiter) {
				r.AllowFunc = func(ctx context.Context, key string) (bool, time.Duration, error) {
					return true, 0, nil
				}
			},
			mockHTTPBehavior: func(m *MockRoundTripper) {
				m.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK}, nil
				}
			},
			mockCheckerRepositoryBehavior: func(c *MockCheckerRepository) {
				c.SaveFunc = func(ctx context.Context, result checker.Result) (*checker.Result, error) {
					return nil, errMockDB
				}
			},
			wantErr: errMockDB,
		},
		{
			name:    "Позитивный: Успешное сохранение упавшего target",
			inputId: 1,
			mockTargetProviderBehavior: func(t *MockTargetProvider) {
				t.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1}, nil
				}
			},
			mockRateLimiterBehavior: func(r *MockrateLimiter) {
				r.AllowFunc = func(ctx context.Context, key string) (bool, time.Duration, error) {
					return true, 0, nil
				}
			},
			mockHTTPBehavior: func(m *MockRoundTripper) {
				m.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return nil, errResponseTimeout
				}
			},
			mockCheckerRepositoryBehavior: func(c *MockCheckerRepository) {
				c.SaveFunc = func(ctx context.Context, result checker.Result) (*checker.Result, error) {
					return &result, nil
				}
			},
			wantResult: checker.Result{
				ID:       0,
				TargetID: 1,
				Status:   checker.StatusDown,
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			mockCheckerRepo := &MockCheckerRepository{}
			if tc.mockCheckerRepositoryBehavior != nil {
				tc.mockCheckerRepositoryBehavior(mockCheckerRepo)
			}

			mockTargetProvider := &MockTargetProvider{}
			if tc.mockTargetProviderBehavior != nil {
				tc.mockTargetProviderBehavior(mockTargetProvider)
			}

			mockrateLimiter := &MockrateLimiter{}
			if tc.mockRateLimiterBehavior != nil {
				tc.mockRateLimiterBehavior(mockrateLimiter)
			}

			mockTransport := &MockRoundTripper{}
			if tc.mockHTTPBehavior != nil {
				tc.mockHTTPBehavior(mockTransport)
			}

			testClient := &http.Client{
				Transport: mockTransport,
			}

			service := checker.NewCheckerService(
				mockTargetProvider,
				mockCheckerRepo,
				mockrateLimiter,
				testClient,
				zap.NewNop(),
			)

			result, _, err := service.CheckManually(context.Background(), tc.inputId)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error: %v, got: %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if result.ID != tc.wantResult.ID || result.TargetID != tc.wantResult.TargetID || result.Status != tc.wantResult.Status {
				t.Fatalf("expected result %+v, got %+v", tc.wantResult, result)
			}
		})
	}
}

func TestCheckerService_CheckScheduled(t *testing.T) {
	type testCase struct {
		name                          string
		inputTarget                   monitor.Target
		mockHTTPBehavior              func(m *MockRoundTripper)
		mockCheckerRepositoryBehavior func(m *MockCheckerRepository)
		wantResult                    checker.Result
		wantLogError                  bool
	}

	var errResponseTimeout = errors.New("dial tcp: i/o timeout")
	var errMockDB = errors.New("simulated db failure")

	tests := []testCase{
		{
			name: "Позитивный: Успешная проверка target",
			inputTarget: monitor.Target{
				ID:             1,
				URL:            "http://example.com",
				Method:         http.MethodGet,
				TimeoutSeconds: 5,
				ExpectedStatus: http.StatusOK,
			},
			mockHTTPBehavior: func(m *MockRoundTripper) {
				m.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
					}, nil
				}
			},
			mockCheckerRepositoryBehavior: func(m *MockCheckerRepository) {
				m.SaveFunc = func(ctx context.Context, result checker.Result) (*checker.Result, error) {
					return &checker.Result{
						ID:     1,
						Status: checker.StatusUp,
					}, nil
				}
			},
			wantResult: checker.Result{
				ID:       0,
				TargetID: 1,
				Status:   checker.StatusUp,
			},
			wantLogError: false,
		},
		{
			name: "Позитивный: Успешное сохранение упавшего target",
			inputTarget: monitor.Target{
				ID:             1,
				URL:            "http://example.com",
				Method:         http.MethodGet,
				TimeoutSeconds: 5,
				ExpectedStatus: http.StatusOK,
			},
			mockHTTPBehavior: func(m *MockRoundTripper) {
				m.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return nil, errResponseTimeout
				}
			},
			mockCheckerRepositoryBehavior: func(m *MockCheckerRepository) {
				m.SaveFunc = func(ctx context.Context, result checker.Result) (*checker.Result, error) {
					return &result, nil
				}
			},
			wantResult: checker.Result{
				ID:       0,
				TargetID: 1,
				Status:   checker.StatusDown,
			},
		},
		{
			name: "Негативный: Ошибка сохранения результата в БД",
			inputTarget: monitor.Target{
				ID:             1,
				URL:            "http://example.com",
				Method:         http.MethodGet,
				TimeoutSeconds: 5,
				ExpectedStatus: http.StatusOK,
			},
			mockHTTPBehavior: func(m *MockRoundTripper) {
				m.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
					}, nil
				}
			},
			mockCheckerRepositoryBehavior: func(m *MockCheckerRepository) {
				m.SaveFunc = func(ctx context.Context, result checker.Result) (*checker.Result, error) {
					return nil, errMockDB
				}
			},
			wantResult: checker.Result{
				ID:       0,
				TargetID: 1,
				Status:   checker.StatusUp,
			},
			wantLogError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			mockCheckerRepo := &MockCheckerRepository{}
			if tc.mockCheckerRepositoryBehavior != nil {
				tc.mockCheckerRepositoryBehavior(mockCheckerRepo)
			}

			mockTransport := &MockRoundTripper{}
			if tc.mockHTTPBehavior != nil {
				tc.mockHTTPBehavior(mockTransport)
			}

			testClient := &http.Client{
				Transport: mockTransport,
			}

			core, recordedLogs := observer.New(zap.ErrorLevel)
			testLogger := zap.New(core)

			service := checker.NewCheckerService(
				nil,
				mockCheckerRepo,
				nil,
				testClient,
				testLogger,
			)

			result := service.CheckScheduled(context.Background(), tc.inputTarget)

			if mockCheckerRepo.SaveCallCount != 1 {
				t.Fatalf("expected Save to be called 1 time, got %d", mockCheckerRepo.SaveCallCount)
			}

			if result.ID != tc.wantResult.ID || result.TargetID != tc.wantResult.TargetID || result.Status != tc.wantResult.Status {
				t.Fatalf("expected result %+v, got %+v", tc.wantResult, result)
			}

			logs := recordedLogs.All()
			if tc.wantLogError {
				if len(logs) != 1 {
					t.Fatalf("expected 1 error log, got %d", len(logs))
				}
				if logs[0].Message != "failed to save check result" {
					t.Errorf("unexpected error log message: %s", logs[0].Message)
				}
			} else {
				if len(logs) > 0 {
					t.Fatalf("expected no error logs, got %d", len(logs))
				}
			}
		})
	}
}

func TestCheckerService_GetCheckHistory(t *testing.T) {
	type testCase struct {
		name                          string
		inputTargetID                 int
		inputLimit                    int
		mockCheckerRepositoryBehavior func(t *testing.T, m *MockCheckerRepository)
		wantResult                    []checker.Result
		wantErr                       error
	}

	var errMockDB = errors.New("simulated database failure")

	tests := []testCase{
		{
			name:          "Позитивный: Успешно получен список",
			inputTargetID: 1,
			inputLimit:    2,
			mockCheckerRepositoryBehavior: func(t *testing.T, m *MockCheckerRepository) {
				m.GetByTargetIDFunc = func(ctx context.Context, targetID, limit int) ([]checker.Result, error) {
					if targetID != 1 {
						t.Errorf("expected targetID 1, got %d", targetID)
					}
					if limit != 2 {
						t.Errorf("expected limit 2, got %d", limit)
					}
					return []checker.Result{
						{ID: 1, TargetID: 1},
						{ID: 2, TargetID: 1},
					}, nil
				}
			},
			wantResult: []checker.Result{
				{ID: 1, TargetID: 1},
				{ID: 2, TargetID: 1},
			},
			wantErr: nil,
		},
		{
			name:          "Позитивный: Записей в базе данных не найдено (пустой результат)",
			inputTargetID: 1,
			inputLimit:    10,
			mockCheckerRepositoryBehavior: func(t *testing.T, m *MockCheckerRepository) {
				m.GetByTargetIDFunc = func(ctx context.Context, targetID, limit int) ([]checker.Result, error) {
					return []checker.Result{}, nil
				}
			},
			wantResult: []checker.Result{},
			wantErr:    nil,
		},
		{
			name:          "Негативный: Невалидный target ID",
			inputTargetID: -1,
			inputLimit:    2,
			mockCheckerRepositoryBehavior: func(t *testing.T, m *MockCheckerRepository) {
				m.GetByTargetIDFunc = func(ctx context.Context, targetID, limit int) ([]checker.Result, error) {
					t.Fatalf("repository method GetByTargetID should not have been called")
					return nil, nil
				}
			},
			wantResult: nil,
			wantErr:    checker.ErrInvalidID,
		},
		{
			name:          "Негативный: Ошибка базы данных",
			inputTargetID: 1,
			inputLimit:    2,
			mockCheckerRepositoryBehavior: func(t *testing.T, m *MockCheckerRepository) {
				m.GetByTargetIDFunc = func(ctx context.Context, targetID, limit int) ([]checker.Result, error) {
					return nil, errMockDB
				}
			},
			wantResult: nil,
			wantErr:    errMockDB,
		},
		{
			name:          "Негативный: Невалидный limit",
			inputTargetID: 1,
			inputLimit:    -1,
			mockCheckerRepositoryBehavior: func(t *testing.T, m *MockCheckerRepository) {
				m.GetByTargetIDFunc = func(ctx context.Context, targetID, limit int) ([]checker.Result, error) {
					t.Fatalf("repository method GetByTargetID should not have been called")
					return nil, nil
				}
			},
			wantResult: nil,
			wantErr:    checker.ErrInvalidLimit,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			mockCheckerRepo := &MockCheckerRepository{}
			if tc.mockCheckerRepositoryBehavior != nil {
				tc.mockCheckerRepositoryBehavior(t, mockCheckerRepo)
			}

			service := checker.NewCheckerService(
				nil,
				mockCheckerRepo,
				nil,
				nil,
				zap.NewNop(),
			)

			result, err := service.GetCheckHistory(context.Background(), tc.inputTargetID, tc.inputLimit)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("epected error: %v, got: %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if len(result) != len(tc.wantResult) {
				t.Fatalf("lenght mismatch: expected %d, got %d", len(tc.wantResult), len(result))
			}

			for i, res := range result {
				if tc.wantResult[i] != res {
					t.Fatalf("mismatch at index %d: expected %+v, got %+v", i, tc.wantResult[i], res)
				}
			}
		})
	}
}
