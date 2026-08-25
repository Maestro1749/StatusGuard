package incident_test

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/incident"
	"StatusGuard/internal/monitor"
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

type MockIncidentRepository struct {
	GetOpenByTargetIDFunc    func(ctx context.Context, targetID int) (*incident.Incident, error)
	CreateFunc               func(ctx context.Context, incident incident.Incident) (*incident.Incident, error)
	IncrementFailureFunc     func(ctx context.Context, incidentID int, LastError *string) error
	ResolveFunc              func(ctx context.Context, incidentID int, resolvedAt time.Time) error
	GetOpenFunc              func(ctx context.Context) ([]incident.Incident, error)
	GetAllOpenByTargetIDFunc func(ctx context.Context, targetID int) ([]incident.Incident, error)
}

func (m *MockIncidentRepository) GetOpenByTargetID(ctx context.Context, targetID int) (*incident.Incident, error) {
	if m.GetOpenByTargetIDFunc != nil {
		return m.GetOpenByTargetIDFunc(ctx, targetID)
	}
	return nil, nil
}

func (m *MockIncidentRepository) Create(ctx context.Context, incident incident.Incident) (*incident.Incident, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, incident)
	}
	return &incident, nil
}

func (m *MockIncidentRepository) IncrementFailure(ctx context.Context, incidentID int, LastError *string) error {
	if m.IncrementFailureFunc != nil {
		return m.IncrementFailureFunc(ctx, incidentID, LastError)
	}
	return nil
}

func (m *MockIncidentRepository) Resolve(ctx context.Context, incidentID int, resolvedAt time.Time) error {
	if m.ResolveFunc != nil {
		return m.ResolveFunc(ctx, incidentID, resolvedAt)
	}
	return nil
}

func (m *MockIncidentRepository) GetOpen(ctx context.Context) ([]incident.Incident, error) {
	if m.GetOpenFunc != nil {
		return m.GetOpenFunc(ctx)
	}
	return nil, nil
}

func (m *MockIncidentRepository) GetAllOpenByTargetID(ctx context.Context, targetID int) ([]incident.Incident, error) {
	if m.GetAllOpenByTargetIDFunc != nil {
		return m.GetAllOpenByTargetIDFunc(ctx, targetID)
	}
	return nil, nil
}

type MockNotifier struct {
	NotifyIncidentOpenedFunc   func(ctx context.Context, name, url, errMsg string) error
	NotifyIncidentResolvedFunc func(ctx context.Context, name, url string) error
}

func (m *MockNotifier) NotifyIncidentOpened(ctx context.Context, name, url, errMsg string) error {
	if m.NotifyIncidentOpenedFunc != nil {
		return m.NotifyIncidentOpenedFunc(ctx, name, url, errMsg)
	}
	return nil
}

func (m *MockNotifier) NotifyIncidentResolved(ctx context.Context, name, url string) error {
	if m.NotifyIncidentResolvedFunc != nil {
		return m.NotifyIncidentResolvedFunc(ctx, name, url)
	}
	return nil
}

func strPtr(s string) *string { return &s }

func TestIncidentService_HandleCheckResult(t *testing.T) {
	errDB := errors.New("database error")
	testErrMsg := "connection timeout"

	type testCase struct {
		name                 string
		target               monitor.Target
		result               checker.Result
		mockRepoBehavior     func(m *MockIncidentRepository)
		mockNotifierBehavior func(m *MockNotifier)
		wantErr              error
	}

	tests := []testCase{
		{
			name:                 "Положительный: неизвестный статус игнорируется",
			target:               monitor.Target{ID: 1},
			result:               checker.Result{Status: "UNKNOWN"},
			mockRepoBehavior:     func(m *MockIncidentRepository) {},
			mockNotifierBehavior: func(m *MockNotifier) {},
			wantErr:              nil,
		},
		{
			name:   "Отрицательный: ошибка при получении инцидента",
			target: monitor.Target{ID: 1},
			result: checker.Result{Status: checker.StatusDown},
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetOpenByTargetIDFunc = func(ctx context.Context, targetID int) (*incident.Incident, error) {
					return nil, errDB
				}
			},
			mockNotifierBehavior: func(m *MockNotifier) {},
			wantErr:              errDB,
		},
		{
			name:   "Положительный: инцидент уже открыт, счётчик увеличивается",
			target: monitor.Target{ID: 1},
			result: checker.Result{Status: checker.StatusDown},
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetOpenByTargetIDFunc = func(ctx context.Context, targetID int) (*incident.Incident, error) {
					return &incident.Incident{ID: 100}, nil
				}
				m.IncrementFailureFunc = func(ctx context.Context, incidentID int, LastError *string) error {
					if incidentID != 100 {
						t.Errorf("expected incident ID 100, got %d", incidentID)
					}
					return nil
				}
			},
			mockNotifierBehavior: func(m *MockNotifier) {},
			wantErr:              nil,
		},
		{
			name:   "Отрицательный: ошибка при открытии инцидента",
			target: monitor.Target{ID: 1},
			result: checker.Result{Status: checker.StatusDown},
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetOpenByTargetIDFunc = func(ctx context.Context, targetID int) (*incident.Incident, error) {
					return nil, nil
				}
				m.CreateFunc = func(ctx context.Context, incident incident.Incident) (*incident.Incident, error) {
					return nil, errDB
				}
			},
			mockNotifierBehavior: func(m *MockNotifier) {},
			wantErr:              errDB,
		},
		{
			name:   "Положительный: открытие инцидента с отправкой уведомления",
			target: monitor.Target{ID: 1, Name: "Test", URL: "http://test.com"},
			result: checker.Result{Status: checker.StatusDown, ErrorMessage: &testErrMsg},
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetOpenByTargetIDFunc = func(ctx context.Context, targetID int) (*incident.Incident, error) {
					return nil, nil
				}
				m.CreateFunc = func(ctx context.Context, incident incident.Incident) (*incident.Incident, error) {
					incident.ID = 100
					return &incident, nil
				}
			},
			mockNotifierBehavior: func(m *MockNotifier) {
				m.NotifyIncidentOpenedFunc = func(ctx context.Context, name, url, errMsg string) error {
					if name != "Test" {
						t.Errorf("expected name 'Test', got: %v", name)
					}
					if url != "http://test.com" {
						t.Errorf("expected URL 'http://test.com', got: %v", url)
					}
					if errMsg != testErrMsg {
						t.Errorf("expected error message '%s', got '%s'", testErrMsg, errMsg)
					}
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name:   "Положительный: инцидент закрывается с отправкой уведомления",
			target: monitor.Target{ID: 1, Name: "Test", URL: "http://test.com"},
			result: checker.Result{Status: checker.StatusUp},
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetOpenByTargetIDFunc = func(ctx context.Context, targetID int) (*incident.Incident, error) {
					return &incident.Incident{ID: 100, StartedAt: time.Now().Add(-time.Hour)}, nil
				}
				m.ResolveFunc = func(ctx context.Context, incidentID int, resolvedAt time.Time) error {
					if incidentID != 100 {
						t.Errorf("expected incident ID 100, got %d", incidentID)
					}
					return nil
				}
			},
			mockNotifierBehavior: func(m *MockNotifier) {
				m.NotifyIncidentResolvedFunc = func(ctx context.Context, name, url string) error {
					return nil
				}
			},
			wantErr: nil,
		},
		{
			name:   "Положительный: нет открытых инцидентов",
			target: monitor.Target{ID: 1},
			result: checker.Result{Status: checker.StatusUp},
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetOpenByTargetIDFunc = func(ctx context.Context, targetID int) (*incident.Incident, error) {
					return nil, nil
				}
			},
			mockNotifierBehavior: func(m *MockNotifier) {},
			wantErr:              nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockIncidentRepository{}
			mockNotifier := &MockNotifier{}

			tc.mockRepoBehavior(mockRepo)
			tc.mockNotifierBehavior(mockNotifier)

			service := incident.NewService(mockRepo, mockNotifier, zap.NewNop())

			err := service.HandleCheckResult(context.Background(), tc.target, tc.result)

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

func TestIncidentService_GetAllOpenByTargetID(t *testing.T) {
	errDB := errors.New("database error")

	type testCase struct {
		name             string
		targetID         int
		mockRepoBehavior func(m *MockIncidentRepository)
		wantErr          error
	}

	tests := []testCase{
		{
			name:             "Отрицательный: невалидный ID (0)",
			targetID:         0,
			mockRepoBehavior: func(m *MockIncidentRepository) {},
			wantErr:          incident.ErrInvalidTargetID,
		},
		{
			name:             "Отрицательный: невалидный ID (-1)",
			targetID:         -1,
			mockRepoBehavior: func(m *MockIncidentRepository) {},
			wantErr:          incident.ErrInvalidTargetID,
		},
		{
			name:     "Отрицательный: ошибка БД",
			targetID: 1,
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetAllOpenByTargetIDFunc = func(ctx context.Context, targetID int) ([]incident.Incident, error) {
					return nil, errDB
				}
			},
			wantErr: errDB,
		},
		{
			name:     "Положительный: успешное получение",
			targetID: 1,
			mockRepoBehavior: func(m *MockIncidentRepository) {
				m.GetAllOpenByTargetIDFunc = func(ctx context.Context, targetID int) ([]incident.Incident, error) {
					return []incident.Incident{{ID: 10, TargetID: 1}}, nil
				}
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockIncidentRepository{}

			tc.mockRepoBehavior(mockRepo)

			service := incident.NewService(mockRepo, &MockNotifier{}, zap.NewNop())

			_, err := service.GetAllOpenByTargetID(context.Background(), tc.targetID)

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
