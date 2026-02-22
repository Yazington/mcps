# MCP Launcher (Go)

This folder contains `run-mcps.go`, a cross-platform Go launcher that starts
shared MCP servers for Tavily, Context7, Playwright, GitHub, test-registrar/test-verifier, Agentation (all via a stdio->HTTP proxy), and an optional Storybook MCP server.

## Prereqs

- `pnpm` on PATH (used to run MCPs and `mcp-proxy`)
- GitHub MCP binary (`github-mcp-server` or `github-mcp-server.exe`) on PATH or in `~/bin`
- Optional for Agentation cloud storage: `AGENTATION_API_KEY`
- Optional for Storybook MCP: a Storybook project with `@storybook/addon-mcp` enabled in `.storybook/main.*`
- Optional fixed Agentation MCP port: set `AGENTATION_MCP_PORT` (defaults to `7017`)
- Optional fixed Storybook port: set `STORYBOOK_PORT` (defaults to `7016`)

## Build the binary

```bash
go build -o run-mcps ./run-mcps.go
```

Note: `run-mcps` locates the test-registrar/test-verifier folders relative to the binary (falling back to the current working directory). Keep `run-mcps` in the repo root or run it from the repo root.

On Windows, you may want `run-mcps.exe`:

```powershell
go build -o run-mcps.exe .\run-mcps.go
```

```powershell
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
# Optional: Storybook project root (if set, Storybook MCP will be started)
$env:STORYBOOK_DIR="E:\work\dg\app"
# Optional: fixed Agentation MCP port (default 7017)
$env:AGENTATION_MCP_PORT="7017"
# Optional: Agentation cloud API key
$env:AGENTATION_API_KEY="ag_..."
$env:STORYBOOK_PORT="7016"
.\run-mcps.exe
```

Windows (cmd.exe):

```cmd
set TAVILY_API_KEY=tvly-...
REM Optional if your Context7 setup requires a key
set CONTEXT7_API_KEY=ctx7-...
set GITHUB_PERSONAL_ACCESS_TOKEN=ghp-...
REM Optional: Storybook project root
set STORYBOOK_DIR=E:\work\dg\app
REM Optional: fixed Agentation MCP port (default 7017)
set AGENTATION_MCP_PORT=7017
REM Optional: Agentation cloud API key
set AGENTATION_API_KEY=ag_...
REM Optional: fixed Storybook port (default 7016)
set STORYBOOK_PORT=7016
run-mcps.exe
```

macOS/Linux (bash/zsh):

```bash
export TAVILY_API_KEY="tvly-..."
# Optional if your Context7 setup requires a key
export CONTEXT7_API_KEY="ctx7-..."
export GITHUB_PERSONAL_ACCESS_TOKEN="ghp-..."
# Optional: Storybook project root
export STORYBOOK_DIR="/path/to/your/storybook/app"
export AGENTATION_MCP_PORT="7017"
# Optional: Agentation cloud API key
export AGENTATION_API_KEY="ag_..."
export STORYBOOK_PORT="7016"
./run-mcps
```

## Optional: pass via flags

```bash
./run-mcps -tavily "tvly-..." -context7 "ctx7-..." -github "ghp-..." -agentation-port 7017 -storybook-dir "/path/to/your/storybook/app" -storybook-port 7016
```

Agentation MCP endpoint:

```text
http://<host>:<agentation-port>/mcp
```

Storybook MCP endpoint (when Storybook is enabled):

```text
http://<host>:<storybook-port>/mcp
```

If `-storybook-dir` / `STORYBOOK_DIR` is not set, `run-mcps` does not launch Storybook itself, but it now checks for an already-running external Storybook MCP endpoint on the configured port and logs that it detected it.
