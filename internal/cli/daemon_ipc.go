package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

const (
	// ReadinessSocketTimeout is how long the parent waits for child to signal readiness
	ReadinessSocketTimeout = 30 * time.Second
)

// DaemonReadinessInfo contains information about a daemon that has successfully started
type DaemonReadinessInfo struct {
	PID             int       `json:"pid"`
	QueueID         string    `json:"queue_id"`
	ControlPlaneURL string    `json:"control_plane_url"`
	WorkerDir       string    `json:"worker_dir"`
	StartTime       time.Time `json:"start_time"`
}

// ReadinessServer runs in the daemon child process and signals readiness to the parent
type ReadinessServer struct {
	socketPath string
}

// NewReadinessServer creates a new readiness server
func NewReadinessServer(socketPath string) *ReadinessServer {
	return &ReadinessServer{
		socketPath: socketPath,
	}
}

// SignalReady signals the parent that the daemon is ready
// This establishes a connection to the parent's socket and sends daemon info
func (rs *ReadinessServer) SignalReady(info DaemonReadinessInfo) error {
	if rs.socketPath == "" {
		// No socket path means we're not running as daemon child, skip signaling
		return nil
	}

	// Connect to parent's socket with timeout
	conn, err := net.DialTimeout("unix", rs.socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to parent socket: %w", err)
	}
	defer conn.Close()

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Send daemon info as JSON
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(info); err != nil {
		return fmt.Errorf("failed to send readiness info: %w", err)
	}

	return nil
}

// ReadinessClient runs in the parent process and waits for the child to signal readiness
type ReadinessClient struct {
	socketPath string
	listener   net.Listener
}

// NewReadinessClient creates a new readiness client
func NewReadinessClient(socketPath string) *ReadinessClient {
	return &ReadinessClient{
		socketPath: socketPath,
	}
}

// WaitForReady waits for the daemon child to signal that it's ready
// Returns the daemon info or an error if the timeout expires
func (rc *ReadinessClient) WaitForReady(timeout time.Duration) (DaemonReadinessInfo, error) {
	var info DaemonReadinessInfo

	// Create Unix socket listener
	listener, err := net.Listen("unix", rc.socketPath)
	if err != nil {
		return info, fmt.Errorf("failed to create readiness socket: %w", err)
	}
	rc.listener = listener

	// Ensure socket is cleaned up
	defer func() {
		listener.Close()
		os.Remove(rc.socketPath)
	}()

	// Accept connection with timeout
	type connResult struct {
		conn net.Conn
		err  error
	}
	connChan := make(chan connResult, 1)

	go func() {
		conn, err := listener.Accept()
		connChan <- connResult{conn: conn, err: err}
	}()

	select {
	case result := <-connChan:
		if result.err != nil {
			return info, fmt.Errorf("failed to accept connection: %w", result.err)
		}
		defer result.conn.Close()

		// Set read deadline
		result.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		// Decode daemon info
		decoder := json.NewDecoder(result.conn)
		if err := decoder.Decode(&info); err != nil {
			return info, fmt.Errorf("failed to decode readiness info: %w", err)
		}

		return info, nil

	case <-time.After(timeout):
		return info, fmt.Errorf("timeout waiting for daemon to start (waited %s)", timeout)
	}
}

// Close closes the readiness client and cleans up resources
func (rc *ReadinessClient) Close() error {
	if rc.listener != nil {
		rc.listener.Close()
	}
	if rc.socketPath != "" {
		os.Remove(rc.socketPath)
	}
	return nil
}
