// Copyright 2026.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolRegister = "register_test_command"
	configEnvVar = "TEST_VERIFIER_CONFIG"
)

type storedConfig struct {
	Command    []string `json:"command"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Env        []string `json:"env,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

type registerArgs struct {
	Command    []string `json:"command" jsonschema:"Command and arguments to run the tests, e.g. [\"npm\",\"test\"]"`
	WorkingDir string   `json:"working_dir,omitempty" jsonschema:"Optional working directory for running the command"`
	Env        []string `json:"env,omitempty" jsonschema:"Optional environment variables as KEY=VALUE"`
}

type registerResult struct {
	ConfigPath string   `json:"config_path"`
	Command    []string `json:"command"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Env        []string `json:"env,omitempty"`
	UpdatedAt  string   `json:"updated_at"`
	Message    string   `json:"message"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-registrar",
		Title:   "Test Command Registrar MCP Server",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Register the test command with register_test_command. This server writes the shared config file used by the test-verifier MCP. Use the TEST_VERIFIER_CONFIG env var to point both servers at the same config path.",
	})

	registerRegisterTool(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("server failed: %v", err)
	}
}

func registerRegisterTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        toolRegister,
		Description: "Register the command used to run tests. Provide the command as an array; the first entry is the executable and remaining entries are args.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args registerArgs) (*mcp.CallToolResult, registerResult, error) {
		command, err := validateCommand(args.Command)
		if err != nil {
			return nil, registerResult{}, err
		}
		env, err := validateEnv(args.Env)
		if err != nil {
			return nil, registerResult{}, err
		}
		if args.WorkingDir != "" {
			info, statErr := os.Stat(args.WorkingDir)
			if statErr != nil {
				return nil, registerResult{}, fmt.Errorf("working_dir does not exist: %w", statErr)
			}
			if !info.IsDir() {
				return nil, registerResult{}, fmt.Errorf("working_dir is not a directory: %s", args.WorkingDir)
			}
		}

		cfgPath, err := configPath()
		if err != nil {
			return nil, registerResult{}, err
		}

		cfg := storedConfig{
			Command:    command,
			WorkingDir: args.WorkingDir,
			Env:        env,
			UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		}

		if err := writeConfig(cfgPath, cfg); err != nil {
			return nil, registerResult{}, err
		}

		message := "Test command registered. The test-verifier MCP can now run tests."
		result := registerResult{
			ConfigPath: cfgPath,
			Command:    cfg.Command,
			WorkingDir: cfg.WorkingDir,
			Env:        cfg.Env,
			UpdatedAt:  cfg.UpdatedAt,
			Message:    message,
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}, result, nil
	})
}

func writeConfig(path string, cfg storedConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		renameErr := os.Rename(tmp, path)
		if renameErr != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("failed to move config into place: %w", err)
		}
	}

	return nil
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
