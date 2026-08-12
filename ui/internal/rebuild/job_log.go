package rebuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// JobFileLogger writes rebuild progress and command output to a text file
// under the work directory so users can share logs for debugging.
type JobFileLogger struct {
	mu       sync.Mutex
	path     string
	latest   string
	f        *os.File
	closed   bool
}

// DefaultLogDir returns {workRoot}/logs (workRoot is typically .../rebuild).
func DefaultLogDir(workRoot string) string {
	if workRoot == "" {
		home, _ := os.UserHomeDir()
		workRoot = filepath.Join(home, ".fridare", "rebuild")
	}
	return filepath.Join(workRoot, "logs")
}

// NewJobFileLogger creates logs/rebuild-YYYYMMDD-HHMMSS.log and logs/latest.log.
func NewJobFileLogger(workRoot string, jobID string) (*JobFileLogger, error) {
	dir := DefaultLogDir(workRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	ts := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("rebuild-%s", ts)
	if jobID != "" {
		// short id suffix
		id := jobID
		if len(id) > 12 {
			id = id[len(id)-12:]
		}
		name = fmt.Sprintf("rebuild-%s-%s", ts, id)
	}
	path := filepath.Join(dir, name+".log")
	latest := filepath.Join(dir, "latest.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	l := &JobFileLogger{path: path, latest: latest, f: f}
	header := fmt.Sprintf("=== Fridare rebuild log ===\nstarted: %s\njob: %s\npath: %s\n\n",
		time.Now().Format(time.RFC3339), jobID, path)
	_, _ = f.WriteString(header)
	_ = f.Sync()
	_ = os.WriteFile(latest, []byte(header), 0644)
	return l, nil
}

// Path returns the primary log file path.
func (l *JobFileLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// LatestPath returns the logs/latest.log path.
func (l *JobFileLogger) LatestPath() string {
	if l == nil {
		return ""
	}
	return l.latest
}

// Log appends a timestamped line.
func (l *JobFileLogger) Log(format string, args ...interface{}) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s %s\n", time.Now().Format("15:04:05.000"), msg)
	l.write(line)
}

// LogBlock appends a multi-line block with a header (e.g. docker output).
func (l *JobFileLogger) LogBlock(title, body string) {
	if l == nil {
		return
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s ----- %s -----\n", time.Now().Format("15:04:05.000"), title))
	body = strings.TrimRight(body, "\n")
	if body != "" {
		b.WriteString(body)
		b.WriteByte('\n')
	}
	b.WriteString("----- end -----\n")
	l.write(b.String())
}

// Close flushes and closes the file; updates latest.log with full content copy.
func (l *JobFileLogger) Close(summary string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if summary != "" {
		_, _ = l.f.WriteString(fmt.Sprintf("\n=== summary ===\n%s\nfinished: %s\n", summary, time.Now().Format(time.RFC3339)))
	}
	_ = l.f.Sync()
	// copy to latest.log
	if data, err := os.ReadFile(l.path); err == nil {
		_ = os.WriteFile(l.latest, data, 0644)
	}
	_ = l.f.Close()
	l.closed = true
}

func (l *JobFileLogger) write(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.f == nil {
		return
	}
	_, _ = l.f.WriteString(s)
	_ = l.f.Sync()
	// keep latest roughly in sync (best-effort append)
	lf, err := os.OpenFile(l.latest, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		_, _ = lf.WriteString(s)
		_ = lf.Close()
	}
}

// LoggingRunner wraps a Runner and records every command + output to the job log.
type LoggingRunner struct {
	Inner  Runner
	Logger *JobFileLogger
}

func (r LoggingRunner) Run(ctx context.Context, env []string, name string, args ...string) (string, error) {
	inner := r.Inner
	if inner == nil {
		inner = ExecRunner{}
	}
	cmdLine := name + " " + strings.Join(args, " ")
	if r.Logger != nil {
		r.Logger.Log("EXEC %s", truncate(cmdLine, 500))
	}
	out, err := inner.Run(ctx, env, name, args...)
	if r.Logger != nil {
		if out != "" {
			r.Logger.LogBlock("cmd output", out)
		}
		if err != nil {
			r.Logger.Log("EXEC FAIL: %v", err)
		} else {
			r.Logger.Log("EXEC OK")
		}
	}
	return out, err
}

