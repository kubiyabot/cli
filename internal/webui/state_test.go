package webui

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewState(t *testing.T) {
	state := NewState()

	assert.NotNil(t, state)
	assert.NotNil(t, state.recentLogs)
	assert.Empty(t, state.workers)
	assert.Empty(t, state.sessions)
	assert.Empty(t, state.recentActivity)
	assert.Empty(t, state.subscribers)
}

func TestRingBuffer_Add(t *testing.T) {
	rb := NewRingBuffer(5)

	// Add entries
	for i := 0; i < 3; i++ {
		rb.Add(mockLogEntry(LogLevelInfo, "message"))
	}

	all := rb.GetAll()
	assert.Len(t, all, 3)
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)

	// Add more than capacity
	for i := 0; i < 10; i++ {
		rb.Add(mockLogEntry(LogLevelInfo, "message"))
	}

	// Should only have capacity entries
	all := rb.GetAll()
	assert.Len(t, all, 3)
}

func TestRingBuffer_GetRecent(t *testing.T) {
	rb := NewRingBuffer(10)

	// Add 5 entries
	for i := 0; i < 5; i++ {
		rb.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     LogLevelInfo,
			Message:   string(rune('A' + i)),
		})
	}

	// Get recent 3
	recent := rb.GetRecent(3)
	assert.Len(t, recent, 3)
	// Most recent should be first
	assert.Equal(t, "E", recent[0].Message)
	assert.Equal(t, "D", recent[1].Message)
	assert.Equal(t, "C", recent[2].Message)
}

func TestRingBuffer_Filter(t *testing.T) {
	rb := NewRingBuffer(10)

	// Add entries with different levels
	rb.Add(LogEntry{Level: LogLevelInfo, Component: "worker", Timestamp: time.Now()})
	rb.Add(LogEntry{Level: LogLevelError, Component: "worker", Timestamp: time.Now()})
	rb.Add(LogEntry{Level: LogLevelInfo, Component: "proxy", Timestamp: time.Now()})
	rb.Add(LogEntry{Level: LogLevelDebug, Component: "proxy", Timestamp: time.Now()})

	// Filter by level
	infoLogs := rb.Filter(LogFilter{Level: LogLevelInfo})
	assert.Len(t, infoLogs, 2)

	// Filter by component
	workerLogs := rb.Filter(LogFilter{Component: "worker"})
	assert.Len(t, workerLogs, 2)

	// Filter by both
	workerInfo := rb.Filter(LogFilter{Level: LogLevelInfo, Component: "worker"})
	assert.Len(t, workerInfo, 1)
}

func TestRingBuffer_ConcurrentAccess(t *testing.T) {
	rb := NewRingBuffer(100)

	var wg sync.WaitGroup
	// Start multiple writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Add(mockLogEntry(LogLevelInfo, "concurrent"))
			}
		}(i)
	}

	// Start multiple readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = rb.GetAll()
				_ = rb.GetRecent(10)
			}
		}()
	}

	wg.Wait()
	// No race conditions should occur
}

func TestState_AddWorker(t *testing.T) {
	state := NewState()

	worker := mockWorker("worker-1")
	state.AddWorker(worker)

	workers := state.GetWorkers()
	require.Len(t, workers, 1)
	assert.Equal(t, "worker-1", workers[0].ID)
}

func TestState_UpdateWorker(t *testing.T) {
	state := NewState()

	// Add initial worker
	worker := mockWorker("worker-1")
	worker.TasksActive = 5
	state.AddWorker(worker)

	// Update with new data
	state.UpdateWorker("worker-1", func(w *WorkerInfo) {
		w.TasksActive = 10
	})

	workers := state.GetWorkers()
	require.Len(t, workers, 1)
	assert.Equal(t, 10, workers[0].TasksActive)
}

func TestState_RemoveWorker(t *testing.T) {
	state := NewState()

	// Add workers
	state.AddWorker(mockWorker("worker-1"))
	state.AddWorker(mockWorker("worker-2"))

	// Remove one
	state.RemoveWorker("worker-1")

	workers := state.GetWorkers()
	require.Len(t, workers, 1)
	assert.Equal(t, "worker-2", workers[0].ID)
}

func TestState_GetWorker(t *testing.T) {
	state := NewState()

	worker := mockWorker("worker-1")
	state.AddWorker(worker)

	found, ok := state.GetWorker("worker-1")
	require.True(t, ok)
	assert.Equal(t, "worker-1", found.ID)

	_, notFound := state.GetWorker("nonexistent")
	assert.False(t, notFound)
}

func TestState_AddSession(t *testing.T) {
	state := NewState()

	session := mockSession("session-1")
	state.AddSession(session)

	sessions := state.GetSessions()
	require.Len(t, sessions, 1)
	assert.Equal(t, "session-1", sessions[0].ID)
}

func TestState_UpdateSession(t *testing.T) {
	state := NewState()

	// Add initial session
	session := mockSession("session-1")
	session.MessagesCount = 5
	state.AddSession(session)

	// Update
	state.UpdateSession("session-1", func(s *SessionInfo) {
		s.MessagesCount = 10
	})

	sessions := state.GetSessions()
	require.Len(t, sessions, 1)
	assert.Equal(t, 10, sessions[0].MessagesCount)
}

func TestState_GetSession(t *testing.T) {
	state := NewState()

	session := mockSession("session-1")
	state.AddSession(session)

	found, ok := state.GetSession("session-1")
	require.True(t, ok)
	assert.Equal(t, "session-1", found.ID)

	_, notFound := state.GetSession("nonexistent")
	assert.False(t, notFound)
}

func TestState_AddLog(t *testing.T) {
	state := NewState()

	state.AddLog(mockLogEntry(LogLevelInfo, "test message"))

	logs := state.GetRecentLogs(10)
	require.Len(t, logs, 1)
	assert.Equal(t, "test message", logs[0].Message)
}

func TestState_AddActivity(t *testing.T) {
	state := NewState()

	state.AddActivity(RecentActivity{
		Type:        "task_completed",
		Description: "Task completed",
		Timestamp:   time.Now(),
	})

	activity := state.GetRecentActivity(10)
	require.Len(t, activity, 1)
	assert.Equal(t, "task_completed", activity[0].Type)
}

func TestState_Subscribe(t *testing.T) {
	state := NewState()

	ch := state.Subscribe()
	assert.NotNil(t, ch)

	// Should have one subscriber
	state.subscribersMu.RLock()
	assert.Len(t, state.subscribers, 1)
	state.subscribersMu.RUnlock()
}

func TestState_Unsubscribe(t *testing.T) {
	state := NewState()

	ch := state.Subscribe()
	state.Unsubscribe(ch)

	state.subscribersMu.RLock()
	assert.Empty(t, state.subscribers)
	state.subscribersMu.RUnlock()
}

func TestState_Broadcast(t *testing.T) {
	state := NewState()

	ch := state.Subscribe()

	// Broadcast event
	event := SSEEvent{Type: SSEEventWorkerUpdate, Data: "data"}
	state.Broadcast(event)

	// Should receive event
	select {
	case received := <-ch:
		assert.Equal(t, SSEEventWorkerUpdate, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive event")
	}
}

func TestState_Broadcast_DropsWhenFull(t *testing.T) {
	state := NewState()

	ch := state.Subscribe()

	// Fill the channel buffer
	for i := 0; i < DefaultSubscriberBuffer+10; i++ {
		state.Broadcast(SSEEvent{Type: SSEEventHeartbeat})
	}

	// Should not block, just drop
	// Verify channel has events
	select {
	case <-ch:
		// Good, received event
	default:
		t.Fatal("channel should have events")
	}
}

func TestState_ConcurrentOperations(t *testing.T) {
	state := NewState()

	var wg sync.WaitGroup

	// Concurrent worker updates
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			state.AddWorker(mockWorker("worker-1"))
		}
	}()

	// Concurrent session adds
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			state.AddSession(mockSession("session-1"))
		}
	}()

	// Concurrent log adds
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			state.AddLog(mockLogEntry(LogLevelInfo, "concurrent"))
		}
	}()

	// Concurrent reads
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = state.GetWorkers()
			_ = state.GetSessions()
			_ = state.GetRecentLogs(10)
		}
	}()

	wg.Wait()
	// No race conditions should occur
}

func TestState_OverviewRecalculation(t *testing.T) {
	state := NewState()

	// Add multiple workers
	w1 := mockWorker("worker-1")
	w1.Status = WorkerStatusRunning
	w1.TasksActive = 3
	state.AddWorker(w1)

	w2 := mockWorker("worker-2")
	w2.Status = WorkerStatusIdle
	w2.TasksActive = 0
	state.AddWorker(w2)

	overview := state.GetOverview()
	assert.Equal(t, 2, overview.TotalWorkers)
	assert.Equal(t, 1, overview.ActiveWorkers)
	assert.Equal(t, 1, overview.IdleWorkers)
	assert.Equal(t, 3, overview.TasksActive)
}

func TestState_ControlPlane(t *testing.T) {
	state := NewState()

	status := ControlPlaneStatus{
		Connected:  true,
		URL:        "https://api.kubiya.ai",
		LatencyMS:  45,
		AuthStatus: "valid",
	}

	state.SetControlPlane(status)

	retrieved := state.GetControlPlane()
	assert.True(t, retrieved.Connected)
	assert.Equal(t, "valid", retrieved.AuthStatus)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{65 * time.Minute, "1h 5m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		assert.Equal(t, tt.expected, result, "duration: %v", tt.duration)
	}
}
