package wsl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	executable := "wsl.exe"
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			return nil, fmt.Errorf("SystemRoot is not set")
		}
		executable = filepath.Join(root, "System32", "wsl.exe")
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.WaitDelay = time.Second
	var stdout, stderr limitedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("WSL command: %w", ctx.Err())
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("WSL command output exceeds 1 MiB")
	}
	if err != nil {
		detail, _ := decodeOutput(stderr.Bytes())
		if detail == "" {
			detail, _ = decodeOutput(stdout.Bytes())
		}
		return nil, fmt.Errorf("WSL command failed: %w: %s", err, detail)
	}
	return stdout.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	const maxOutput = 1024 * 1024
	n := len(p)
	remaining := maxOutput - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.Buffer.Write(p)
	return n, nil
}
