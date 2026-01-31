// Copyright 2026.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolRun               = "run_tests"
	defaultTimeoutSeconds = 600
	configEnvVar          = "TEST_VERIFIER_CONFIG"
)

type storedConfig struct {
	Command    []string `json:"command"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Env        []string `json:"env,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

type runArgs struct {
	ExtraArgs      []string `json:"extra_args,omitempty" jsonschema:"Additional arguments appended to the registered command"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"Optional timeout in seconds (default 600)"`
	Env            []string `json:"env,omitempty" jsonschema:"Extra environment variables for this run (KEY=VALUE)"`
}

type runResult struct {
	ConfigPath string   `json:"config_path"`
	Command    []string `json:"command"`
	WorkingDir string   `json:"working_dir,omitempty"`
	ExitCode   int      `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	Success    bool     `json:"success"`
	TimedOut   bool     `json:"timed_out"`
	Error      string   `json:"error,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-verifier",
		Title:   "Test Verifier MCP Server",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Run tests with run_tests. The test command is loaded from the shared config file (set by the test-registrar MCP). Use the TEST_VERIFIER_CONFIG env var to point both servers at the same config path.",
	})

	registerRunTool(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("server failed: %v", err)
	}
}

func registerRunTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        toolRun,
		Description: "Run the registered test command and return stdout, stderr, and exit status.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args runArgs) (*mcp.CallToolResult, runResult, error) {
		cfg, cfgPath, err := loadConfig()
		if err != nil {
			return nil, runResult{}, err
		}

		extraArgs, err := validateCommand(args.ExtraArgs)
		if err != nil && len(args.ExtraArgs) > 0 {
			return nil, runResult{}, fmt.Errorf("extra_args: %w", err)
		}

		cmdline := append([]string{}, cfg.Command...)
		if len(extraArgs) > 0 {
			cmdline = append(cmdline, extraArgs...)
		}

		runEnv, err := validateEnv(args.Env)
		if err != nil {
			return nil, runResult{}, err
		}

		timeoutSeconds := args.TimeoutSeconds
		if timeoutSeconds <= 0 {
			timeoutSeconds = defaultTimeoutSeconds
		}

		start := time.Now()
		runCtx := ctx
		var cancel context.CancelFunc
		if timeoutSeconds > 0 {
			runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
			defer cancel()
		}

		cmd := exec.CommandContext(runCtx, cmdline[0], cmdline[1:]...)
		if cfg.WorkingDir != "" {
			cmd.Dir = cfg.WorkingDir
		}

		if len(cfg.Env) > 0 || len(runEnv) > 0 {
			cmd.Env = append(os.Environ(), cfg.Env...)
			cmd.Env = append(cmd.Env, runEnv...)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Start()
		if err != nil {
			result := runResult{
				ConfigPath: cfgPath,
				Command:    cmdline,
				WorkingDir: cfg.WorkingDir,
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				Success:    false,
				Error:      err.Error(),
				UpdatedAt:  cfg.UpdatedAt,
			}
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, result, nil
		}

		err = cmd.Wait()
		duration := time.Since(start)
		result := runResult{
			ConfigPath: cfgPath,
			Command:    cmdline,
			WorkingDir: cfg.WorkingDir,
			DurationMs: duration.Milliseconds(),
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			Success:    true,
			UpdatedAt:  cfg.UpdatedAt,
		}

		if err != nil {
			result.Success = false
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				result.TimedOut = true
				result.Error = fmt.Sprintf("timed out after %d seconds", timeoutSeconds)
			}

			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = -1
				if result.Error == "" {
					result.Error = err.Error()
				}
			}
		} else if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
			if result.ExitCode != 0 {
				result.Success = false
			}
		}

		summary := fmt.Sprintf("Test run finished with exit code %d.", result.ExitCode)
		if result.TimedOut {
			summary = fmt.Sprintf("Test run timed out after %d seconds.", timeoutSeconds)
		} else if !result.Success && result.ExitCode == -1 && result.Error != "" {
			summary = fmt.Sprintf("Test run failed to start: %s", result.Error)
		}

		toolResult := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
		if result.ExitCode == -1 && result.Error != "" {
			toolResult.IsError = true
		}

		return toolResult, result, nil
	})
}

func loadConfig() (storedConfig, string, error) {
	path, err := configPath()
	if err != nil {
		return storedConfig{}, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return storedConfig{}, path, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg storedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return storedConfig{}, path, fmt.Errorf("failed to parse config: %w", err)
	}

	command, err := validateCommand(cfg.Command)
	if err != nil {
		return storedConfig{}, path, fmt.Errorf("invalid command in config: %w", err)
	}
	cfg.Command = command

	env, err := validateEnv(cfg.Env)
	if err != nil {
		return storedConfig{}, path, fmt.Errorf("invalid env in config: %w", err)
	}
	cfg.Env = env

	if cfg.WorkingDir != "" {
		info, statErr := os.Stat(cfg.WorkingDir)
		if statErr != nil {
			return storedConfig{}, path, fmt.Errorf("working_dir does not exist: %w", statErr)
		}
		if !info.IsDir() {
			return storedConfig{}, path, fmt.Errorf("working_dir is not a directory: %s", cfg.WorkingDir)
		}
	}

	return cfg, path, nil
}

func configPath() (string, error) {
	path := strings.TrimSpace(os.Getenv(configEnvVar))
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(cwd, ".test-verifier", "command.json")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func validateCommand(command []string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("command must contain at least one element")
	}
	clean := make([]string, 0, len(command))
	for _, part := range command {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("command entries cannot be empty")
		}
		clean = append(clean, trimmed)
	}
	return clean, nil
}

func validateEnv(env []string) ([]string, error) {
	if len(env) == 0 {
		return nil, nil
	}
	clean := make([]string, 0, len(env))
	for _, entry := range env {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("env entries must be KEY=VALUE, got %q", entry)
		}
		clean = append(clean, trimmed)
	}
	return clean, nil
}
