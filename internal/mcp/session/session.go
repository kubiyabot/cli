package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kubiyabot/cli/internal/sentry"
)

// State holds per-session state
type State struct {
	ID          string
	UserID      string
	Email       string
	Permissions []string
	Settings    map[string]interface{}
	StartTime   time.Time
	LastActive  time.Time
	Metadata    map[string]interface{}
	mutex       sync.RWMutex
}

// Manager manages client sessions
type Manager struct {
	sessions map[string]*State
	mutex    sync.RWMutex
	timeout  time.Duration
}

// NewManager creates a new session manager
func NewManager(sessionTimeout time.Duration) *Manager {
	m := &Manager{
		sessions: make(map[string]*State),
		timeout:  sessionTimeout,
	}

	// Start cleanup goroutine
	go m.cleanupExpiredSessions()

	return m
}

// CreateSession creates a new session
func (m *Manager) CreateSession(sessionID, userID, email string, permissions []string) (*State, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if session already exists
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	session := &State{
		ID:          sessionID,
		UserID:      userID,
		Email:       email,
		Permissions: permissions,
		Settings:    make(map[string]interface{}),
		StartTime:   time.Now(),
		LastActive:  time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	m.sessions[sessionID] = session

	// Add breadcrumb for debugging
	sentry.AddBreadcrumb("session", "Session created", map[string]interface{}{
		"session_id": sessionID,
		"user_id":    userID,
		"email":      email,
	})

	return session, nil
}

// GetSession retrieves a session
func (m *Manager) GetSession(sessionID string) (*State, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	session, exists := m.sessions[sessionID]
	if exists {
		// Update last active time
		session.mutex.Lock()
		session.LastActive = time.Now()
		session.mutex.Unlock()
	}

	return session, exists
}

// UpdateSession updates session data
func (m *Manager) UpdateSession(sessionID string, updateFn func(*State)) error {
	m.mutex.RLock()
	session, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	updateFn(session)
	session.LastActive = time.Now()

	return nil
}

// RemoveSession removes a session
func (m *Manager) RemoveSession(sessionID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		sentry.AddBreadcrumb("session", "Session removed", map[string]interface{}{
			"session_id": sessionID,
			"duration":   time.Since(session.StartTime).String(),
		})
	}

	delete(m.sessions, sessionID)
}

// GetAllSessions returns all active sessions
func (m *Manager) GetAllSessions() map[string]*State {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a copy to avoid concurrent access issues
	sessions := make(map[string]*State, len(m.sessions))
	for k, v := range m.sessions {
		sessions[k] = v
	}

	return sessions
}

// cleanupExpiredSessions removes inactive sessions
func (m *Manager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mutex.Lock()
		now := time.Now()

		for sessionID, session := range m.sessions {
			session.mutex.RLock()
			lastActive := session.LastActive
			session.mutex.RUnlock()

			if now.Sub(lastActive) > m.timeout {
				delete(m.sessions, sessionID)

				sentry.AddBreadcrumb("session", "Session expired", map[string]interface{}{
					"session_id": sessionID,
					"idle_time":  now.Sub(lastActive).String(),
				})
			}
		}

		m.mutex.Unlock()
	}
}

// HasPermission checks if session has a specific permission
func (s *State) HasPermission(permission string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, p := range s.Permissions {
		if p == permission || p == "admin" { // Admin has all permissions
			return true
		}
	}

	return false
}

// SetSetting sets a session setting
func (s *State) SetSetting(key string, value interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Settings[key] = value
	s.LastActive = time.Now()
}

// GetSetting retrieves a session setting
func (s *State) GetSetting(key string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, exists := s.Settings[key]
	return value, exists
}

// SessionFromContext retrieves session from context
func SessionFromContext(ctx context.Context) (*State, bool) {
	session, ok := ctx.Value("session").(*State)
	return session, ok
}

// ContextWithSession adds session to context
func ContextWithSession(ctx context.Context, session *State) context.Context {
	return context.WithValue(ctx, "session", session)
}
