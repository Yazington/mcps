# MCP Launcher (Go)

This folder contains `run-mcps.go`, a cross-platform Go launcher that starts
shared MCP servers for Tavily, Context7, Playwright, GitHub, and the test-registrar/test-verifier pair via a stdio→HTTP proxy.

## Prereqs

- `pnpm` on PATH (used to run MCPs and `mcp-proxy`)
- GitHub MCP binary (`github-mcp-server` or `github-mcp-server.exe`) on PATH or in `~/bin`

## Build the binary

```bash
go build -o run-mcps ./run-mcps.go
```

Note: `run-mcps` locates the test-registrar/test-verifier folders relative to the binary (falling back to the current working directory). Keep `run-mcps` in the repo root or run it from the repo root.

On Windows, you may want `run-mcps.exe`:

```powershell
go build -o run-mcps.exe .\run-mcps.go
```

```
git clone https://github.com/github/github-mcp-server
cd github-mcp-server\cmd\github-mcp-server
go build -o $env:USERPROFILE\bin\github-mcp-server.exe
```

## Example: pass env vars from host

Windows (PowerShell):

```powershell
$env:TAVILY_API_KEY="tvly-..."
# Optional if your Context7 setup requires a key
$env:CONTEXT7_API_KEY="ctx7-..."
$env:GITHUB_PERSONAL_ACCESS_TOKEN="ghp-..."
.\run-mcps.exe
```

Windows (cmd.exe):

```cmd
set TAVILY_API_KEY=tvly-...
REM Optional if your Context7 setup requires a key
set CONTEXT7_API_KEY=ctx7-...
set GITHUB_PERSONAL_ACCESS_TOKEN=ghp-...
run-mcps.exe
```

macOS/Linux (bash/zsh):

```bash
export TAVILY_API_KEY="tvly-..."
# Optional if your Context7 setup requires a key
export CONTEXT7_API_KEY="ctx7-..."
export GITHUB_PERSONAL_ACCESS_TOKEN="ghp-..."
./run-mcps
```

## Optional: pass via flags

```bash
./run-mcps -tavily "tvly-..." -context7 "ctx7-..." -github "ghp-..."
```
