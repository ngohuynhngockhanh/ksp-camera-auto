package mcp

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"sync"
)

// RunStdio runs the MCP server over standard input/output until ctx is cancelled.
// It redirects standard logging to os.Stderr to guarantee stdout carries only valid JSON-RPC frames.
func (s *Server) RunStdio(ctx context.Context) error {
	log.SetOutput(os.Stderr)
	return RunStdioWithStreams(ctx, s, os.Stdin, os.Stdout)
}

// RunStdioWithStreams runs the MCP JSON-RPC message loop using arbitrary reader and writer streams.
func RunStdioWithStreams(ctx context.Context, s *Server, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Support large JSON payloads (e.g. large inventory / monitor list) up to 8MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 8*1024*1024)

	var writeMu sync.Mutex

	lines := make(chan string)
	scanErr := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanErr <- scanner.Err()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-scanErr:
			if err != nil && err != io.EOF {
				return err
			}
			return nil

		case line, ok := <-lines:
			if !ok {
				return nil
			}
			if len(line) == 0 {
				continue
			}

			respBytes, isNotification, err := s.ProcessMessage(ctx, []byte(line))
			if err != nil {
				log.Printf("mcp stdio process error: %v", err)
				continue
			}
			if isNotification || len(respBytes) == 0 {
				continue
			}

			writeMu.Lock()
			_, writeErr := out.Write(append(respBytes, '\n'))
			writeMu.Unlock()
			if writeErr != nil {
				return writeErr
			}
		}
	}
}
