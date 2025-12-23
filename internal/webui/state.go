package webui

import (
	"fmt"
	"sync"
	"time"
)

const (
	// MaxRecentLogs is the maximum number of logs to keep in memory
	MaxRecentLogs = 1000
	// MaxRecentActivity is the maximum number of recent activities to keep
	MaxRecentActivity = 50
	// DefaultSubscriberBuffer is the buffer size for SSE subscribers
	DefaultSubscriberBuffer = 100
)

// RingBuffer is a thread-safe circular buffer for logs
type RingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	head    int
	size    int
	maxSize int
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		entries: make([]LogEntry, capacity),
		maxSize: capacity,
	}
}

// Add adds an entry to the ring buffer
func (rb *RingBuffer) Add(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.maxSize
	if rb.size < rb.maxSize {
		rb.size++
	}
}

// GetAll returns all entries in order (oldest first)
func (rb *RingBuffer) GetAll() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]LogEntry, rb.size)
	if rb.size == 0 {
		return result
	}

	start := 0
	if rb.size == rb.maxSize {
		start = rb.head
	}

	for i := 0; i < rb.size; i++ {
		idx := (start + i) % rb.maxSize
		result[i] = rb.entries[idx]
	}

	return result
}

// GetRecent returns the most recent n entries (newest first)
func (rb *RingBuffer) GetRecent(n int) []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > rb.size {
		n = rb.size
	}

	result := make([]LogEntry, n)
	for i := 0; i < n; i++ {
		idx := (rb.head - 1 - i + rb.maxSize) % rb.maxSize
		result[i] = rb.entries[idx]
	}

	return result
}

// Filter returns entries matching the filter criteria
func (rb *RingBuffer) Filter(filter LogFilter) []LogEntry {
	all := rb.GetAll()
	result := make([]LogEntry, 0)

	for _, entry := range all {
		if filter.Level != "" && entry.Level != filter.Level {
			continue
		}
		if filter.Component != "" && entry.Component != filter.Component {
			continue
		}
		if filter.WorkerID != "" && entry.WorkerID != filter.WorkerID {
			continue
		}
		if filter.Since != nil && entry.Timestamp.Before(*filter.Since) {
			continue
		}
		// Note: Search filtering should be done at a higher level for better performance
		result = append(result, entry)
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[len(result)-filter.Limit:]
	}

	return result
}

// State holds the in-memory state for the WebUI
type State struct {
	mu sync.RWMutex

	// Core state
	startTime    time.Time
	config       WorkerConfig
	overview     WorkerPoolOverview
	controlPlane ControlPlaneStatus

	// Workers
	workers       map[string]*WorkerInfo
	workerMetrics map[string]*WorkerMetrics

	// Sessions
	sessions map[string]*SessionInfo

	// Logs
	recentLogs *RingBuffer

	// Recent activity
	recentActivity []RecentActivity

	// Event broadcasting
	subscribersMu sync.RWMutex
	subscribers   map[chan SSEEvent]struct{}
}

// NewState creates a new state instance
func NewState() *State {
	return &State{
		startTime:      time.Now(),
		workers:        make(map[string]*WorkerInfo),
		workerMetrics:  make(map[string]*WorkerMetrics),
		sessions:       make(map[string]*SessionInfo),
		recentLogs:     NewRingBuffer(MaxRecentLogs),
		recentActivity: make([]RecentActivity, 0, MaxRecentActivity),
		subscribers:    make(map[chan SSEEvent]struct{}),
	}
}

// SetConfig sets the worker configuration
func (s *State) SetConfig(config WorkerConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetConfig returns the current configuration
func (s *State) GetConfig() WorkerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetOverview returns the current overview
func (s *State) GetOverview() WorkerPoolOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	overview := s.overview
	overview.Uptime = time.Since(s.startTime)
	overview.UptimeFormatted = formatDuration(overview.Uptime)
	overview.StartTime = s.startTime

	return overview
}

// UpdateOverview updates the overview state
func (s *State) UpdateOverview(update func(*WorkerPoolOverview)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(&s.overview)
}

// GetControlPlane returns the control plane status
func (s *State) GetControlPlane() ControlPlaneStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.controlPlane
}

// SetControlPlane sets the control plane status
func (s *State) SetControlPlane(status ControlPlaneStatus) {
	s.mu.Lock()
	s.controlPlane = status
	s.overview.ControlPlaneOK = status.Connected
	s.mu.Unlock()

	s.Broadcast(SSEEvent{
		Type: SSEEventControlPlane,
		Data: status,
	})
}

// AddWorker adds or updates a worker
func (s *State) AddWorker(worker *WorkerInfo) {
	s.mu.Lock()
	s.workers[worker.ID] = worker
	s.recalculateOverview()
	s.mu.Unlock()

	s.Broadcast(SSEEvent{
		Type: SSEEventWorkerUpdate,
		Data: worker,
	})
}

// UpdateWorker updates a specific worker
func (s *State) UpdateWorker(id string, update func(*WorkerInfo)) {
	s.mu.Lock()
	if worker, ok := s.workers[id]; ok {
		update(worker)
		s.recalculateOverview()
	}
	s.mu.Unlock()
}

// RemoveWorker removes a worker
func (s *State) RemoveWorker(id string) {
	s.mu.Lock()
	delete(s.workers, id)
	delete(s.workerMetrics, id)
	s.recalculateOverview()
	s.mu.Unlock()
}

// GetWorker returns a specific worker
func (s *State) GetWorker(id string) (*WorkerInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	worker, ok := s.workers[id]
	if !ok {
		return nil, false
	}
	// Return a copy
	workerCopy := *worker
	return &workerCopy, true
}

// GetWorkers returns all workers
func (s *State) GetWorkers() []WorkerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]WorkerInfo, 0, len(s.workers))
	for _, w := range s.workers {
		workers = append(workers, *w)
	}
	return workers
}

// SetWorkerMetrics sets metrics for a worker
func (s *State) SetWorkerMetrics(id string, metrics *WorkerMetrics) {
	s.mu.Lock()
	s.workerMetrics[id] = metrics
	s.mu.Unlock()

	s.Broadcast(SSEEvent{
		Type: SSEEventMetrics,
		Data: map[string]interface{}{
			"worker_id": id,
			"metrics":   metrics,
		},
	})
}

// GetWorkerMetrics returns metrics for a worker
func (s *State) GetWorkerMetrics(id string) (*WorkerMetrics, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	metrics, ok := s.workerMetrics[id]
	if !ok {
		return nil, false
	}
	metricsCopy := *metrics
	return &metricsCopy, true
}

// AddSession adds or updates a session
func (s *State) AddSession(session *SessionInfo) {
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	s.Broadcast(SSEEvent{
		Type: SSEEventSession,
		Data: session,
	})
}

// UpdateSession updates a specific session
func (s *State) UpdateSession(id string, update func(*SessionInfo)) {
	s.mu.Lock()
	if session, ok := s.sessions[id]; ok {
		update(session)
	}
	s.mu.Unlock()
}

// RemoveSession removes a session
func (s *State) RemoveSession(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// GetSession returns a specific session
func (s *State) GetSession(id string) (*SessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	sessionCopy := *session
	return &sessionCopy, true
}

// GetSessions returns all sessions
func (s *State) GetSessions() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]SessionInfo, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, *sess)
	}
	return sessions
}

// AddLog adds a log entry
func (s *State) AddLog(entry LogEntry) {
	s.recentLogs.Add(entry)

	s.Broadcast(SSEEvent{
		Type: SSEEventLog,
		Data: entry,
	})
}

// GetLogs returns filtered logs
func (s *State) GetLogs(filter LogFilter) []LogEntry {
	return s.recentLogs.Filter(filter)
}

// GetRecentLogs returns the most recent logs
func (s *State) GetRecentLogs(n int) []LogEntry {
	return s.recentLogs.GetRecent(n)
}

// AddActivity adds a recent activity entry
func (s *State) AddActivity(activity RecentActivity) {
	s.mu.Lock()
	s.recentActivity = append([]RecentActivity{activity}, s.recentActivity...)
	if len(s.recentActivity) > MaxRecentActivity {
		s.recentActivity = s.recentActivity[:MaxRecentActivity]
	}
	s.mu.Unlock()
}

// GetRecentActivity returns recent activity entries
func (s *State) GetRecentActivity(n int) []RecentActivity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n > len(s.recentActivity) {
		n = len(s.recentActivity)
	}

	result := make([]RecentActivity, n)
	copy(result, s.recentActivity[:n])
	return result
}

// Subscribe creates a new SSE subscription
func (s *State) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, DefaultSubscriberBuffer)
	s.subscribersMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subscribersMu.Unlock()
	return ch
}

// Unsubscribe removes an SSE subscription
func (s *State) Unsubscribe(ch chan SSEEvent) {
	s.subscribersMu.Lock()
	delete(s.subscribers, ch)
	close(ch)
	s.subscribersMu.Unlock()
}

// Broadcast sends an event to all subscribers
func (s *State) Broadcast(event SSEEvent) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Drop event if subscriber is slow
		}
	}
}

// BroadcastOverview broadcasts the current overview
func (s *State) BroadcastOverview() {
	s.Broadcast(SSEEvent{
		Type: SSEEventOverview,
		Data: s.GetOverview(),
	})
}

// recalculateOverview recalculates overview metrics from workers
// Must be called with mu held
func (s *State) recalculateOverview() {
	s.overview.TotalWorkers = len(s.workers)
	s.overview.ActiveWorkers = 0
	s.overview.IdleWorkers = 0
	s.overview.TasksActive = 0
	s.overview.TasksProcessed = 0

	for _, worker := range s.workers {
		switch worker.Status {
		case WorkerStatusRunning, WorkerStatusBusy:
			s.overview.ActiveWorkers++
		case WorkerStatusIdle:
			s.overview.IdleWorkers++
		}
		s.overview.TasksActive += worker.TasksActive
		s.overview.TasksProcessed += worker.TasksTotal
	}
}

// GetMetricsSnapshot returns a point-in-time metrics snapshot
func (s *State) GetMetricsSnapshot() MetricsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := MetricsSnapshot{
		Timestamp: time.Now(),
		Workers:   make([]WorkerMetrics, 0, len(s.workerMetrics)),
	}

	for _, metrics := range s.workerMetrics {
		snapshot.Workers = append(snapshot.Workers, *metrics)
		snapshot.TotalCPU += metrics.CPUPercent
		snapshot.TotalMemoryMB += metrics.MemoryMB
	}

	return snapshot
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		m := d / time.Minute
		s := (d % time.Minute) / time.Second
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}
