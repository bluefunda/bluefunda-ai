package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

// Client manages a bai subprocess for programmatic interaction.
type Client struct {
	opts Options
	mu   sync.Mutex
	cmd  *exec.Cmd
}

// NewClient creates a new SDK client with the given options.
func NewClient(opts Options) *Client {
	if opts.BinaryPath == "" {
		opts.BinaryPath = "bai"
	}
	if opts.MaxTurns == 0 {
		opts.MaxTurns = 20
	}
	return &Client{opts: opts}
}

// Send executes a prompt and streams typed events back on the returned channel.
// The channel is closed when the subprocess completes or the context is cancelled.
func (c *Client) Send(ctx context.Context, prompt string) (<-chan Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--max-turns", strconv.Itoa(c.opts.MaxTurns),
	}
	if c.opts.Model != "" {
		args = append(args, "--model", c.opts.Model)
	}
	if c.opts.AutoApprove {
		args = append(args, "--auto")
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, c.opts.BinaryPath, args...)
	if c.opts.WorkDir != "" {
		cmd.Dir = c.opts.WorkDir
	}
	cmd.Env = append(os.Environ(), c.opts.Env...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start bai: %w", err)
	}
	c.cmd = cmd

	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			var ev Event
			if json.Unmarshal(scanner.Bytes(), &ev) == nil {
				select {
				case ch <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
		_ = cmd.Wait()
	}()

	return ch, nil
}

// Stop terminates the running subprocess, if any.
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}
