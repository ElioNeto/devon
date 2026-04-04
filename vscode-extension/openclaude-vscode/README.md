# OpenClaude VS Code Extension

A sleek VS Code companion for OpenClaude with a visual **Control Center** plus terminal-first workflows.

## Features

- **Control Center sidebar UI** in the Activity Bar:
  - Launch OpenClaude
  - Open repository/docs
  - Open VS Code theme picker
- **Terminal launch command**: `OpenClaude: Launch in Terminal`
- **Built-in dark theme**: `OpenClaude Terminal Black` (terminal-inspired, low-glare, neon accents)

## Requirements

- VS Code `1.95+`
- `openclaude` available in your terminal PATH (`npm install -g @gitlawb/openclaude`)

## Commands

- `OpenClaude: Open Control Center`
- `OpenClaude: Launch in Terminal`
- `OpenClaude: Open Repository`

## Settings

- `openclaude.launchCommand` (default: `openclaude`)
- `openclaude.terminalName` (default: `OpenClaude`)
- `openclaude.useOpenAIShim` (default: `true`)

## Development

From this folder:

```bash
npm run lint
```

To package (optional):

```bash
npm run package
```

