package lifecycle

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// LifecycleTestSuite provides isolated Redis and PostgreSQL containers for lifecycle tests
// Matches pattern from 11-RESEARCH.md: testcontainers for reproducible state management
type LifecycleTestSuite struct {
	suite.Suite

	// Containers
	redisContainer    testcontainers.Container
	postgresContainer *postgres.PostgresContainer

	// Clients
	redisClient    *redis.Client
	postgresClient *pgx.Conn

	// Connection info
	redisHost    string
	postgresHost string
	postgresUser string
	postgresPass string
	postgresDB   string

	// Test infrastructure
	logger       *zap.Logger
	testDataDir  string
	binaryCache  map[string]string // binary name → built path
}

// SetupSuite runs once before all tests
// Starts containers, creates clients, runs schema migrations
func (s *LifecycleTestSuite) SetupSuite() {
	ctx := context.Background()

	// Create test logger (suppress debug noise in test output)
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	logger, err := loggerConfig.Build()
	s.Require().NoError(err)
	s.logger = logger

	s.logger.Info("Starting testcontainers suite")

	// Start Redis container
	s.logger.Info("Starting Redis container")
	redisContainer, err := rediscontainer.Run(ctx,
		"redis:7-alpine",
		rediscontainer.WithSnapshotting(10, 1),
		rediscontainer.WithLogLevel(rediscontainer.LogLevelVerbose),
	)
	s.Require().NoError(err, "Failed to start Redis container")
	s.redisContainer = redisContainer

	// Get Redis connection string
	redisHost, err := redisContainer.Host(ctx)
	s.Require().NoError(err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	s.Require().NoError(err)
	s.redisHost = fmt.Sprintf("%s:%s", redisHost, redisPort.Port())

	// Create Redis client
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: s.redisHost,
	})
	err = s.redisClient.Ping(ctx).Err()
	s.Require().NoError(err, "Redis should be reachable")

	s.logger.Info("Redis container started", zap.String("host", s.redisHost))

	// Start PostgreSQL container
	s.logger.Info("Starting PostgreSQL container")
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("allchat_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	s.Require().NoError(err, "Failed to start PostgreSQL container")
	s.postgresContainer = pgContainer

	// Get PostgreSQL connection info
	pgHost, err := pgContainer.Host(ctx)
	s.Require().NoError(err)
	pgPort, err := pgContainer.MappedPort(ctx, "5432")
	s.Require().NoError(err)
	s.postgresHost = fmt.Sprintf("%s:%s", pgHost, pgPort.Port())
	s.postgresUser = "test"
	s.postgresPass = "test"
	s.postgresDB = "allchat_test"

	// Create PostgreSQL connection
	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s",
		s.postgresUser, s.postgresPass, s.postgresHost, s.postgresDB)
	pgConn, err := pgx.Connect(ctx, connStr)
	s.Require().NoError(err, "PostgreSQL connection should succeed")
	s.postgresClient = pgConn

	err = pgConn.Ping(ctx)
	s.Require().NoError(err, "PostgreSQL should be reachable")

	s.logger.Info("PostgreSQL container started", zap.String("host", s.postgresHost))

	// Run schema migrations
	s.runSchemaMigrations(ctx)

	// Create temp directory for test data (mock responses, etc.)
	tmpDir, err := os.MkdirTemp("", "lifecycle-test-*")
	s.Require().NoError(err)
	s.testDataDir = tmpDir

	// Initialize binary cache
	s.binaryCache = make(map[string]string)

	s.logger.Info("Testcontainers suite setup complete")
}

// TearDownSuite runs once after all tests
// Terminates containers and cleans up resources
func (s *LifecycleTestSuite) TearDownSuite() {
	ctx := context.Background()

	s.logger.Info("Tearing down testcontainers suite")

	// Close clients
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.postgresClient != nil {
		s.postgresClient.Close(ctx)
	}

	// Terminate containers
	if s.redisContainer != nil {
		if err := s.redisContainer.Terminate(ctx); err != nil {
			s.logger.Warn("Failed to terminate Redis container", zap.Error(err))
		}
	}
	if s.postgresContainer != nil {
		if err := s.postgresContainer.Terminate(ctx); err != nil {
			s.logger.Warn("Failed to terminate PostgreSQL container", zap.Error(err))
		}
	}

	// Clean up test data directory
	if s.testDataDir != "" {
		os.RemoveAll(s.testDataDir)
	}

	s.logger.Info("Testcontainers suite teardown complete")
}

// SetupTest runs before each test
// Flushes Redis, truncates PostgreSQL tables for clean state
func (s *LifecycleTestSuite) SetupTest() {
	ctx := context.Background()

	// Flush Redis database
	err := s.redisClient.FlushDB(ctx).Err()
	s.Require().NoError(err, "Redis flush should succeed")

	// Truncate PostgreSQL tables
	_, err = s.postgresClient.Exec(ctx, "TRUNCATE sources, overlays CASCADE")
	s.Require().NoError(err, "PostgreSQL truncate should succeed")

	s.logger.Debug("Test state reset complete")
}

// runSchemaMigrations creates required tables for source-manager integration
func (s *LifecycleTestSuite) runSchemaMigrations(ctx context.Context) {
	s.logger.Info("Running schema migrations")

	// Create overlays table (required by sources foreign key)
	_, err := s.postgresClient.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS overlays (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	s.Require().NoError(err, "Failed to create overlays table")

	// Create sources table (matches source-manager schema)
	_, err = s.postgresClient.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
			platform TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			stream_id TEXT,
			is_active BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(overlay_id, platform, channel_id)
		)
	`)
	s.Require().NoError(err, "Failed to create sources table")

	// Create index for active source queries
	_, err = s.postgresClient.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_sources_active
		ON sources(platform, is_active) WHERE is_active = true
	`)
	s.Require().NoError(err, "Failed to create sources index")

	s.logger.Info("Schema migrations complete")
}

// BuildListener builds a listener binary and returns the path
// Caches builds to avoid rebuilding for each test
func (s *LifecycleTestSuite) BuildListener(name string) string {
	// Check cache
	if path, exists := s.binaryCache[name]; exists {
		return path
	}

	s.logger.Info("Building listener binary", zap.String("name", name))

	// Determine service path
	projectRoot := s.getProjectRoot()
	var servicePath string
	switch name {
	case "youtube-listener":
		servicePath = filepath.Join(projectRoot, "services", "youtube-listener")
	case "youtube-listener-innertube":
		servicePath = filepath.Join(projectRoot, "services", "youtube-listener-innertube")
	default:
		s.Require().Fail("Unknown listener binary", "name: %s", name)
	}

	// Build binary to temp directory
	binaryPath := filepath.Join(s.testDataDir, name)
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd")
	cmd.Dir = servicePath
	output, err := cmd.CombinedOutput()
	s.Require().NoError(err, "Build failed for %s: %s", name, string(output))

	// Cache path
	s.binaryCache[name] = binaryPath

	s.logger.Info("Binary built successfully",
		zap.String("name", name),
		zap.String("path", binaryPath),
	)

	return binaryPath
}

// StartListener starts a listener subprocess with test environment
// Returns the command handle for lifecycle control and log capture
func (s *LifecycleTestSuite) StartListener(binaryPath string, env map[string]string) (*exec.Cmd, *ListenerHandle, error) {
	s.logger.Info("Starting listener subprocess", zap.String("binary", binaryPath))

	// Build environment variables
	envVars := os.Environ()

	// Inject testcontainer connection URLs
	testEnv := map[string]string{
		"REDIS_HOST":        s.redisHost,
		"DATABASE_HOST":     s.postgresHost,
		"DATABASE_USER":     s.postgresUser,
		"DATABASE_PASSWORD": s.postgresPass,
		"DATABASE_NAME":     s.postgresDB,
		"LOG_LEVEL":         "info",
	}

	// Merge with provided environment
	for k, v := range env {
		testEnv[k] = v
	}

	// Convert to KEY=VALUE format
	for k, v := range testEnv {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Create command
	cmd := exec.Command(binaryPath)
	cmd.Env = envVars

	// Capture stdout/stderr for log analysis
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start listener: %w", err)
	}

	// Create handle for log capture
	handle := &ListenerHandle{
		cmd:    cmd,
		stdout: stdoutPipe,
		stderr: stderrPipe,
		logger: s.logger,
	}
	handle.startLogCapture()

	s.logger.Info("Listener subprocess started",
		zap.Int("pid", cmd.Process.Pid),
	)

	return cmd, handle, nil
}

// WaitForReady polls the listener's /health/ready endpoint until it returns 200 OK
// Returns error if timeout exceeded or listener crashes
func (s *LifecycleTestSuite) WaitForReady(handle *ListenerHandle, timeout time.Duration) error {
	s.logger.Debug("Waiting for listener readiness", zap.Duration("timeout", timeout))

	deadline := time.Now().Add(timeout)

	// Find available port by checking listener logs for "Starting server on :"
	// For now, assume default port 8080
	port := 8080

	healthURL := fmt.Sprintf("http://localhost:%d/health/ready", port)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if process still running
			if handle.cmd.ProcessState != nil && handle.cmd.ProcessState.Exited() {
				return fmt.Errorf("listener exited before becoming ready")
			}

			// Try health check
			resp, err := http.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					s.logger.Info("Listener ready")
					return nil
				}
			}

			// Check timeout
			if time.Now().After(deadline) {
				return fmt.Errorf("listener readiness timeout exceeded")
			}
		}
	}
}

// StopListener gracefully stops a listener subprocess
// Sends SIGTERM and waits for process to exit
func (s *LifecycleTestSuite) StopListener(handle *ListenerHandle) error {
	s.logger.Info("Stopping listener subprocess", zap.Int("pid", handle.cmd.Process.Pid))

	// Send SIGTERM
	if err := handle.cmd.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- handle.cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process exited
		handle.stopLogCapture()
		s.logger.Info("Listener stopped cleanly")
		return err
	case <-time.After(10 * time.Second):
		// Timeout - force kill
		handle.cmd.Process.Kill()
		handle.stopLogCapture()
		return fmt.Errorf("listener shutdown timeout - process killed")
	}
}

// getProjectRoot finds the project root directory
func (s *LifecycleTestSuite) getProjectRoot() string {
	// Walk up from current directory until we find go.mod
	dir, err := os.Getwd()
	s.Require().NoError(err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			s.Require().Fail("Could not find project root (no go.work found)")
		}
		dir = parent
	}
}

// GetFreePort finds an available TCP port
func (s *LifecycleTestSuite) GetFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	s.Require().NoError(err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// InsertTestOverlay creates a test overlay in PostgreSQL
func (s *LifecycleTestSuite) InsertTestOverlay(ctx context.Context, name string) string {
	var overlayID string
	err := s.postgresClient.QueryRow(ctx,
		"INSERT INTO overlays (name) VALUES ($1) RETURNING id",
		name,
	).Scan(&overlayID)
	s.Require().NoError(err)
	return overlayID
}

// InsertTestSource creates a test source in PostgreSQL
func (s *LifecycleTestSuite) InsertTestSource(ctx context.Context, overlayID, platform, channelID string, isActive bool) string {
	var sourceID string
	err := s.postgresClient.QueryRow(ctx,
		"INSERT INTO sources (overlay_id, platform, channel_id, is_active) VALUES ($1, $2, $3, $4) RETURNING id",
		overlayID, platform, channelID, isActive,
	).Scan(&sourceID)
	s.Require().NoError(err)
	return sourceID
}

// UpdateSourceStatus updates a source's active status
func (s *LifecycleTestSuite) UpdateSourceStatus(ctx context.Context, sourceID string, isActive bool) {
	_, err := s.postgresClient.Exec(ctx,
		"UPDATE sources SET is_active = $1, updated_at = NOW() WHERE id = $2",
		isActive, sourceID,
	)
	s.Require().NoError(err)
}

// GetRedisStreamLength returns the number of messages in a Redis stream
func (s *LifecycleTestSuite) GetRedisStreamLength(ctx context.Context, streamKey string) int64 {
	length, err := s.redisClient.XLen(ctx, streamKey).Result()
	s.Require().NoError(err)
	return length
}

// ListenerHandle provides access to a running listener subprocess
type ListenerHandle struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
	logger *zap.Logger

	// Log capture
	stdoutBuf []byte
	stderrBuf []byte
	stopLog   chan struct{}
}

// startLogCapture starts capturing stdout/stderr in background
func (h *ListenerHandle) startLogCapture() {
	h.stopLog = make(chan struct{})

	// Capture stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-h.stopLog:
				return
			default:
				n, err := h.stdout.Read(buf)
				if n > 0 {
					h.stdoutBuf = append(h.stdoutBuf, buf[:n]...)
				}
				if err != nil {
					return
				}
			}
		}
	}()

	// Capture stderr
	go func() {
		buf := make([]byte, 4096)
		for {
			select {
			case <-h.stopLog:
				return
			default:
				n, err := h.stderr.Read(buf)
				if n > 0 {
					h.stderrBuf = append(h.stderrBuf, buf[:n]...)
				}
				if err != nil {
					return
				}
			}
		}
	}()
}

// stopLogCapture stops log capture goroutines
func (h *ListenerHandle) stopLogCapture() {
	if h.stopLog != nil {
		close(h.stopLog)
	}
}

// GetStdout returns captured stdout as string
func (h *ListenerHandle) GetStdout() string {
	return string(h.stdoutBuf)
}

// GetStderr returns captured stderr as string
func (h *ListenerHandle) GetStderr() string {
	return string(h.stderrBuf)
}

// ContainsLog checks if captured logs contain a substring
func (h *ListenerHandle) ContainsLog(substr string) bool {
	stdout := string(h.stdoutBuf)
	stderr := string(h.stderrBuf)
	return contains(stdout, substr) || contains(stderr, substr)
}

// contains is a helper to check substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && anySubstring(s, substr))
}

func anySubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
