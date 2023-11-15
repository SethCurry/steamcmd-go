package steamcmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const prompt = "\u001b[1m\nSteam\u003e\u001b[0m"

func splitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	asStr := string(data)

	nextNewline := strings.Index(asStr, "\n")
	nextPrompt := strings.Index(asStr, prompt)

	useNewline := false

	if nextNewline == -1 && nextPrompt == -1 {
		return 0, nil, nil
	} else if nextNewline >= 0 && nextPrompt >= 0 {
		if nextNewline < nextPrompt {
			useNewline = true
		}
		useNewline = false
	} else if nextNewline >= 0 {
		useNewline = true
	}

	if useNewline {
		return nextNewline + 1, []byte(asStr[:nextNewline]), nil
	}

	if nextPrompt == 0 {
		return len(prompt), []byte(asStr[:len(prompt)]), nil
	}

	return nextPrompt, []byte(asStr[:nextPrompt]), nil
}

func copyLinesToChan(reader io.ReadCloser, ch chan string, doPrintln bool) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(splitFunc)
	for scanner.Scan() {
		err := scanner.Err()
		if err == nil {
			txt := scanner.Text()
			ch <- txt
			if doPrintln {
				fmt.Println("stdout: " + txt)
			}
			continue
		}

		contents := scanner.Bytes()
		if string(contents) == "\u001b[1m\nSteam\u003e\u001b[0m" {
			ch <- string(contents)
			if doPrintln {
				fmt.Println("stdout: " + string(contents))
			}
		}
	}
}

func copyChanToWriter(writer io.WriteCloser, ch chan string, doPrintln bool) {
	for {
		line := <-ch
		if doPrintln {
			fmt.Println("stdin: " + line)
		}
		writer.Write([]byte(line + "\n"))
	}
}

func NewSessionIO(ctx context.Context) (*SessionIO, error) {
	cmd := exec.CommandContext(ctx, "/usr/games/steamcmd")

	stdinLines := make(chan string)
	stdoutLines := make(chan string, 100)
	stderrLines := make(chan string, 100)

	stdinSend, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	go copyChanToWriter(stdinSend, stdinLines, true)

	stdoutRecv, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	go copyLinesToChan(stdoutRecv, stdoutLines, true)

	stderrRecv, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	go copyLinesToChan(stderrRecv, stderrLines, true)

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start steamcmd session")
	}

	sess := &SessionIO{
		cmd:    cmd,
		stdin:  stdinLines,
		stdout: stdoutLines,
		stderr: stderrLines,
		lock:   sync.Mutex{},
	}

	_, err = sess.WaitForPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to wait for prompt: %w", err)
	}

	return sess, nil
}

type SessionIO struct {
	cmd    *exec.Cmd
	stdin  chan string
	stdout <-chan string
	stderr <-chan string
	lock   sync.Mutex
}

type Output struct {
	Stdout []string
	Stderr []string
}

func (s *SessionIO) WaitForSuffix(sfx string) (*Output, error) {
	stdoutLines := []string{}
	stderrLines := []string{}

	for {
		gotPrompt := false
		select {
		case newStdout := <-s.stdout:
			stdoutLines = append(stdoutLines, newStdout)
			if strings.HasSuffix(newStdout, sfx) {
				gotPrompt = true
			}
		case newStderr := <-s.stderr:
			stderrLines = append(stderrLines, newStderr)
		}

		if gotPrompt {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}

	return &Output{
		Stdout: stdoutLines,
		Stderr: stderrLines,
	}, nil
}

func (s *SessionIO) WaitForPrompt() (*Output, error) {
	return s.WaitForSuffix("\u001b[1m\nSteam\u003e\u001b[0m")
}

func (s *SessionIO) Exec(command string) (*Output, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.stdin <- command

	cmdOut, err := s.WaitForPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to gather output: %w", err)
	}

	if len(cmdOut.Stderr) != 0 {
		return nil, fmt.Errorf("failed to execute command: %s", strings.Join(cmdOut.Stderr, "\n"))
	}

	return cmdOut, nil
}

func (s *SessionIO) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.stdin <- "quit"
	time.Sleep(time.Millisecond * 100)
	s.cmd.Wait()

	return nil
}
