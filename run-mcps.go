package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

type procSpec struct {
	name string
	cmd  []string
	env  []string
}

func main() {
	tavilyKey := flag.String("tavily", os.Getenv("TAVILY_API_KEY"), "Tavily API key")
	context7Key := flag.String("context7", os.Getenv("CONTEXT7_API_KEY"), "Context7 API key (optional)")
	githubToken := flag.String("github", githubEnvToken(), "GitHub token")
	host := flag.String("host", "127.0.0.1", "Bind host for proxy")
	basePort := flag.Int("port", 7010, "Base port (tavily uses base, then +1,+2,+3)")
	flag.Parse()

	if *tavilyKey == "" || *githubToken == "" {
		log.Println("Tavily:", *tavilyKey != "")
		log.Println("GitHub:", *githubToken != "")
		log.Fatal("Missing required keys. Set TAVILY_API_KEY and GITHUB_PERSONAL_ACCESS_TOKEN (or GITHUB_API_KEY) or pass flags.")
	}

	// Each MCP is stdio-based; mcp-proxy exposes them over HTTP/SSE.
	githubPath := githubBinary()
	if githubPath == "" {
		log.Fatal("GitHub MCP binary not found. Build it and add to PATH or place it in ~/bin (github-mcp-server or github-mcp-server.exe).")
	}
	specs := []procSpec{
		{
			name: "tavily",
			cmd:  []string{"pnpm", "dlx", "mcp-proxy", "--host", *host, "--port", fmt.Sprintf("%d", *basePort), "--", "pnpm", "dlx", "tavily-mcp@latest"},
			env:  []string{"TAVILY_API_KEY=" + *tavilyKey},
		},
		{
			name: "context7",
			cmd:  context7Command(*host, *basePort+1, *context7Key),
			env:  nil,
		},
		{
			name: "playwright",
			cmd:  []string{"pnpm", "dlx", "mcp-proxy", "--host", *host, "--port", fmt.Sprintf("%d", *basePort+2), "--", "pnpm", "dlx", "@playwright/mcp@latest"},
			env:  nil,
		},
		{
			name: "github",
			cmd:  []string{"pnpm", "dlx", "mcp-proxy", "--host", *host, "--port", fmt.Sprintf("%d", *basePort+3), "--", githubPath, "stdio"},
			env:  []string{"GITHUB_PERSONAL_ACCESS_TOKEN=" + *githubToken},
		},
	}

	procs := make([]*exec.Cmd, 0, len(specs))
	for _, spec := range specs {
		cmd := exec.Command(spec.cmd[0], spec.cmd[1:]...)
		cmd.Env = append(os.Environ(), spec.env...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Fatalf("failed to start %s: %v", spec.name, err)
		}
		log.Printf("started %s on port %d (pid=%d)", spec.name, portFor(spec.name, *basePort), cmd.Process.Pid)
		procs = append(procs, cmd)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")

	for _, cmd := range procs {
		_ = cmd.Process.Signal(os.Interrupt)
	}
	time.Sleep(2 * time.Second)
	for _, cmd := range procs {
		_ = cmd.Process.Kill()
	}
}

func githubBinary() string {
	name := "github-mcp-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	// Prefer PATH; otherwise fall back to ~/bin.
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	fallback := filepath.Join(home, "bin", name)
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}

func portFor(name string, base int) int {
	switch name {
	case "tavily":
		return base
	case "context7":
		return base + 1
	case "playwright":
		return base + 2
	case "github":
		return base + 3
	default:
		return base
	}
}

func context7Command(host string, port int, key string) []string {
	base := []string{"pnpm", "dlx", "mcp-proxy", "--host", host, "--port", fmt.Sprintf("%d", port), "--", "pnpm", "dlx", "@upstash/context7-mcp"}
	if key == "" {
		return base
	}
	return append(base, "--api-key", key)
}

func githubEnvToken() string {
	if v := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN"); v != "" {
		return v
	}
	return os.Getenv("GITHUB_API_KEY")
}
