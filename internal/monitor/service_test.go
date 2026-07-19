package monitor_test

import (
	"StatusGuard/internal/monitor"
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

type MockMonitorRepository struct {
	CreateTargetFunc  func(ctx context.Context, target monitor.Target) (*monitor.Target, error)
	DeleteTargetFunc  func(ctx context.Context, id int) error
	GetAllTargetsFunc func(ctx context.Context) ([]monitor.Target, error)
	GetByIDFunc       func(ctx context.Context, id int) (*monitor.Target, error)
	UpdateTargetFunc  func(ctx context.Context, target monitor.Target) (*monitor.Target, error)
}

func (m *MockMonitorRepository) CreateTarget(ctx context.Context, target monitor.Target) (*monitor.Target, error) {
	return m.CreateTargetFunc(ctx, target)
}

func (m *MockMonitorRepository) DeleteTarget(ctx context.Context, id int) error {
	return m.DeleteTargetFunc(ctx, id)
}

func (m *MockMonitorRepository) GetAllTargets(ctx context.Context) ([]monitor.Target, error) {
	return m.GetAllTargetsFunc(ctx)
}

func (m *MockMonitorRepository) GetByID(ctx context.Context, id int) (*monitor.Target, error) {
	return m.GetByIDFunc(ctx, id)
}

func (m *MockMonitorRepository) UpdateTarget(ctx context.Context, target monitor.Target) (*monitor.Target, error) {
	return m.UpdateTargetFunc(ctx, target)
}

func TestMonitorService_CreateTarget(t *testing.T) {
	type testCase struct {
		name                  string
		targetName            string
		targetURL             string
		targetMethod          string
		targetExpectedStatus  int
		targetIntervalSeconds int
		targetTimeoutSeconds  int
		mockBehavior          func(m *MockMonitorRepository)
		wantErr               error
	}

	tests := []testCase{
		{
			name:         "Негативный: пустое имя",
			targetName:   "	",
			targetURL:    "https://google.com",
			wantErr:      monitor.ErrEmptyName,
			mockBehavior: func(m *MockMonitorRepository) {},
		},
		{
			name:         "Негативный: невалидный URL",
			targetName:   "Google",
			targetURL:    "http:google.com",
			wantErr:      monitor.ErrInvalidURL,
			mockBehavior: func(m *MockMonitorRepository) {},
		},
		{
			name:         "Негативный: метод не GET",
			targetName:   "Google",
			targetURL:    "https://google.com",
			targetMethod: "POST",
			wantErr:      monitor.ErrInvalidMethod,
			mockBehavior: func(m *MockMonitorRepository) {},
		},
		{
			name:                  "Негативный: таймаут больше половины интервала",
			targetName:            "Google",
			targetURL:             "https://google.com",
			targetIntervalSeconds: 10,
			targetTimeoutSeconds:  6,
			wantErr:               monitor.ErrInvalidTimeout,
			mockBehavior:          func(m *MockMonitorRepository) {},
		},
		{
			name:                  "Позитивный: успешное создание",
			targetName:            "Google",
			targetURL:             "https://google.com",
			targetMethod:          "GET",
			targetIntervalSeconds: 60,
			targetTimeoutSeconds:  5,
			wantErr:               nil,
			mockBehavior: func(m *MockMonitorRepository) {
				m.CreateTargetFunc = func(ctx context.Context, target monitor.Target) (*monitor.Target, error) {
					if target.Method != "GET" {
						t.Errorf("expected method GET, got %s", target.Method)
					}
					return &monitor.Target{ID: 1, Name: target.Name}, nil
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockMonitorRepository{}
			tc.mockBehavior(mockRepo)

			service := monitor.NewMonitorService(mockRepo, zap.NewNop())

			_, err := service.CreateTarget(
				context.Background(),
				tc.targetName,
				tc.targetURL,
				tc.targetMethod,
				tc.targetExpectedStatus,
				tc.targetIntervalSeconds,
				tc.targetTimeoutSeconds,
			)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error: %v, got: %v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestMonitorService_UpdateTarget(t *testing.T) {
	type testCase struct {
		name         string
		input        monitor.UpdateTargetInput
		mockBehavior func(m *MockMonitorRepository)
		wantErr      error
	}

	prtInt := func(i int) *int { return &i }
	prtStr := func(s string) *string { return &s }

	tests := []testCase{
		{
			name: "Негативный: target не найден в БД",
			input: monitor.UpdateTargetInput{
				ID: 22,
			},
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return nil, monitor.ErrTargetNotFound
				}
			},
			wantErr: monitor.ErrTargetNotFound,
		},
		{
			name: "Негативный: невалидное новое имя",
			input: monitor.UpdateTargetInput{
				ID:   1,
				Name: prtStr("	"),
			},
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1, Name: "Old name"}, nil
				}
			},
			wantErr: monitor.ErrEmptyName,
		},
		{
			name: "Негативный: правило timeout*2 > interval нарушено старым интервалом из БД",
			input: monitor.UpdateTargetInput{
				ID:             1,
				TimeoutSeconds: prtInt(10),
			},
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1, IntervalSeconds: 15, TimeoutSeconds: 5}, nil
				}
			},
			wantErr: monitor.ErrInvalidTimeout,
		},
		{
			name: "Позитивный: успешное обновление одного поля",
			input: monitor.UpdateTargetInput{
				ID:   1,
				Name: prtStr("New Name"),
			},
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return &monitor.Target{ID: 1, Name: "Old Name", IntervalSeconds: 60, TimeoutSeconds: 5}, nil
				}
				m.UpdateTargetFunc = func(ctx context.Context, target monitor.Target) (*monitor.Target, error) {
					if target.Name != "New Name" {
						t.Errorf("expected name to be updated to 'New Name', got %s", target.Name)
					}
					return &target, nil
				}
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockMonitorRepository{}
			tc.mockBehavior(mockRepo)

			service := monitor.NewMonitorService(mockRepo, zap.NewNop())

			_, err := service.UpdateTarget(context.Background(), tc.input)

			if tc.wantErr != nil {
				if err == nil || !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error: %v, got: %v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestMonitorService_DeleteTarget(t *testing.T) {
	type testCase struct {
		name         string
		inputID      int
		mockBehavior func(m *MockMonitorRepository)
		wantErr      error
	}

	tests := []testCase{
		{
			name:    "Позитивный: успешное удаление target",
			inputID: 1,
			mockBehavior: func(m *MockMonitorRepository) {
				m.DeleteTargetFunc = func(ctx context.Context, id int) error {
					if id != 1 {
						t.Errorf("expected id 1, got %d", id)
					}
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name:         "Негативный: невалидный id",
			inputID:      -12,
			mockBehavior: func(m *MockMonitorRepository) {},
			wantErr:      monitor.ErrInvalidID,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockMonitorRepository{}
			tc.mockBehavior(mockRepo)

			service := monitor.NewMonitorService(mockRepo, zap.NewNop())

			err := service.DeleteTarget(context.Background(), tc.inputID)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error: %v, got: %v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestMonitorService_GetTarget(t *testing.T) {
	type testCase struct {
		name         string
		inputID      int
		mockBehavior func(m *MockMonitorRepository)
		wantErr      error
	}

	tests := []testCase{
		{
			name:    "Позитивный: успешно получен target",
			inputID: 1,
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					if id != 1 {
						t.Errorf("expected id 1, got: %v", id)
					}
					return &monitor.Target{ID: id}, nil
				}
			},
			wantErr: nil,
		},
		{
			name:         "Негативный: невалидный id",
			inputID:      0,
			mockBehavior: func(m *MockMonitorRepository) {},
			wantErr:      monitor.ErrInvalidID,
		},
		{
			name:    "Негативный: ошибка репозитория (not found)",
			inputID: 22,
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetByIDFunc = func(ctx context.Context, id int) (*monitor.Target, error) {
					return nil, monitor.ErrTargetNotFound
				}
			},
			wantErr: monitor.ErrTargetNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockMonitorRepository{}
			tc.mockBehavior(mockRepo)

			service := monitor.NewMonitorService(mockRepo, zap.NewNop())

			_, err := service.GetTarget(context.Background(), tc.inputID)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error: %v, got: %v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestMonitorService_GetAllTargets(t *testing.T) {
	type testCase struct {
		name         string
		mockBehavior func(m *MockMonitorRepository)
		wantResult   []monitor.Target
		wantErr      error
	}

	tests := []testCase{
		{
			name: "Позитивный: успешно получены targets",
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetAllTargetsFunc = func(ctx context.Context) ([]monitor.Target, error) {
					return []monitor.Target{
						{ID: 1, Name: "Google"},
						{ID: 2, Name: "Yandex"},
					}, nil
				}
			},
			wantResult: []monitor.Target{
				{ID: 1, Name: "Google"},
				{ID: 2, Name: "Yandex"},
			},
			wantErr: nil,
		},
		{
			name: "Негативный: ошибка базы данных",
			mockBehavior: func(m *MockMonitorRepository) {
				m.GetAllTargetsFunc = func(ctx context.Context) ([]monitor.Target, error) {
					return nil, monitor.ErrInternalServer
				}
			},
			wantResult: nil,
			wantErr:    monitor.ErrInternalServer,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockMonitorRepository{}
			tc.mockBehavior(mockRepo)

			service := monitor.NewMonitorService(mockRepo, zap.NewNop())

			res, err := service.GetAllTargets(context.Background())

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error: %v, got: %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if len(res) != len(tc.wantResult) {
				t.Fatalf("expected result length: %d, got: %d", len(tc.wantResult), len(res))
			}

			for i := range res {
				if res[i].ID != tc.wantResult[i].ID || res[i].Name != tc.wantResult[i].Name {
					t.Errorf("mismatch at index %d: expected %+v, got %+v", i, tc.wantResult[i], res[i])
				}
			}
		})
	}
}
