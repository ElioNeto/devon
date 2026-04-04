const vscode = require('vscode');
const crypto = require('crypto');
const { exec } = require('child_process');
const { promisify } = require('util');

const execAsync = promisify(exec);
const OPENCLAUDE_REPO_URL = 'https://github.com/Gitlawb/openclaude';

async function isCommandAvailable(command) {
  try {
    if (!command) {
      return false;
    }

    if (process.platform === 'win32') {
      await execAsync(`where ${command}`);
    } else {
      await execAsync(`command -v ${command}`);
    }

    return true;
  } catch {
    return false;
  }
}

function getExecutableFromCommand(command) {
  return command.trim().split(/\s+/)[0];
}

async function launchOpenClaude() {
  const configured = vscode.workspace.getConfiguration('openclaude');
  const launchCommand = configured.get('launchCommand', 'openclaude');
  const terminalName = configured.get('terminalName', 'OpenClaude');
  const shimEnabled = configured.get('useOpenAIShim', false);
  const executable = getExecutableFromCommand(launchCommand);
  const installed = await isCommandAvailable(executable);

  if (!installed) {
    const action = await vscode.window.showErrorMessage(
      `OpenClaude command not found: ${executable}. Install it with: npm install -g @gitlawb/openclaude`,
      'Open Repository'
    );

    if (action === 'Open Repository') {
      await vscode.env.openExternal(vscode.Uri.parse(OPENCLAUDE_REPO_URL));
    }

    return;
  }

  const env = {};
  if (shimEnabled) {
    env.CLAUDE_CODE_USE_OPENAI = '1';
  }

  const terminal = vscode.window.createTerminal({
    name: terminalName,
    env,
  });

  terminal.show(true);
  terminal.sendText(launchCommand, true);
}

class OpenClaudeControlCenterProvider {
  async resolveWebviewView(webviewView) {
    webviewView.webview.options = { enableScripts: true };
    const configured = vscode.workspace.getConfiguration('openclaude');
    const launchCommand = configured.get('launchCommand', 'openclaude');
    const executable = getExecutableFromCommand(launchCommand);
    const installed = await isCommandAvailable(executable);
    const shimEnabled = configured.get('useOpenAIShim', false);
    const shortcut = process.platform === 'darwin' ? 'Cmd+Shift+P' : 'Ctrl+Shift+P';

    webviewView.webview.html = this.getHtml(webviewView.webview, {
      installed,
      shimEnabled,
      shortcut,
      executable,
    });

    webviewView.webview.onDidReceiveMessage(async (message) => {
      if (message?.type === 'launch') {
        await launchOpenClaude();
        return;
      }

      if (message?.type === 'docs') {
        await vscode.env.openExternal(vscode.Uri.parse(OPENCLAUDE_REPO_URL));
        return;
      }

      if (message?.type === 'commands') {
        await vscode.commands.executeCommand('workbench.action.showCommands');
      }
    });
  }

  getHtml(webview, status) {
    const nonce = crypto.randomBytes(16).toString('base64');
    const runtimeLabel = status.installed ? 'available' : 'missing';
    const shimLabel = status.shimEnabled ? 'enabled (CLAUDE_CODE_USE_OPENAI=1)' : 'disabled';
    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <style>
    :root {
      --oc-bg-1: #081018;
      --oc-bg-2: #0e1b29;
      --oc-line: #2f4d63;
      --oc-accent: #7fffd4;
      --oc-accent-dim: #4db89a;
      --oc-text-dim: #94a7b5;
    }
    * {
      margin: 0;
      padding: 0;
      box-sizing: border-box;
    }
    body {
      font-family: "Cascadia Code", "JetBrains Mono", "Fira Code", Consolas, "Liberation Mono", Menlo, monospace;
      color: var(--vscode-foreground);
      background:
        radial-gradient(circle at 85% -10%, color-mix(in srgb, var(--oc-accent) 16%, transparent), transparent 45%),
        linear-gradient(165deg, var(--oc-bg-1), var(--oc-bg-2));
      padding: 14px;
      min-height: 100vh;
      line-height: 1.45;
      letter-spacing: 0.15px;
      overflow-x: hidden;
    }
    .panel {
      border: 1px solid color-mix(in srgb, var(--oc-line) 80%, var(--vscode-editorWidget-border));
      border-radius: 10px;
      background: color-mix(in srgb, var(--oc-bg-1) 78%, var(--vscode-sideBar-background));
      box-shadow: 0 0 0 1px rgba(127, 255, 212, 0.08), 0 10px 24px rgba(0, 0, 0, 0.35);
      overflow: hidden;
      animation: boot 360ms ease-out;
    }
    .topbar {
      padding: 8px 10px;
      font-size: 10px;
      text-transform: uppercase;
      color: var(--oc-text-dim);
      border-bottom: 1px solid var(--oc-line);
      background: color-mix(in srgb, var(--oc-bg-2) 74%, black);
      display: flex;
      justify-content: space-between;
      gap: 8px;
    }
    .boot-dot {
      color: var(--oc-accent);
      animation: blink 1.2s steps(1, end) infinite;
    }
    .content {
      padding: 12px;
      display: grid;
      gap: 14px;
    }
    .title {
      color: var(--oc-accent);
      font-size: 14px;
      font-weight: 700;
      margin-bottom: 4px;
    }
    .sub {
      color: var(--oc-text-dim);
      font-size: 11px;
    }
    .terminal-box {
      border: 1px dashed color-mix(in srgb, var(--oc-line) 78%, white);
      border-radius: 8px;
      padding: 10px;
      background: color-mix(in srgb, var(--oc-bg-2) 78%, black);
      font-size: 11px;
      display: grid;
      gap: 6px;
    }
    .terminal-row {
      color: var(--oc-text-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .prompt {
      color: var(--oc-accent);
    }
    .cursor::after {
      content: "_";
      animation: blink 1s steps(1, end) infinite;
      margin-left: 1px;
    }
    .actions {
      display: grid;
      gap: 8px;
    }
    .btn {
      width: 100%;
      border: 1px solid var(--oc-line);
      border-radius: 7px;
      padding: 10px;
      cursor: pointer;
      text-align: left;
      font-family: inherit;
      font-size: 11px;
      letter-spacing: 0.3px;
      text-transform: uppercase;
      transition: transform 140ms ease, border-color 140ms ease, background 140ms ease;
      background: color-mix(in srgb, var(--oc-bg-2) 82%, black);
      color: var(--vscode-foreground);
      position: relative;
      overflow: hidden;
    }
    .btn::before {
      content: ">";
      color: var(--oc-accent-dim);
      margin-right: 8px;
      display: inline-block;
      width: 10px;
    }
    .btn:hover {
      border-color: var(--oc-accent-dim);
      transform: translateX(2px);
      background: color-mix(in srgb, var(--oc-bg-2) 68%, #113642);
    }
    .btn.primary {
      border-color: color-mix(in srgb, var(--oc-accent) 50%, var(--oc-line));
      box-shadow: inset 0 0 0 1px rgba(127, 255, 212, 0.12);
    }
    .hint {
      font-size: 10px;
      color: var(--oc-text-dim);
      border-top: 1px solid var(--oc-line);
      padding-top: 10px;
    }
    .hint code {
      font-family: inherit;
      color: var(--oc-accent);
      background: rgba(0, 0, 0, 0.26);
      padding: 2px 5px;
      border-radius: 4px;
      border: 1px solid rgba(127, 255, 212, 0.14);
    }
    @keyframes blink {
      50% {
        opacity: 0;
      }
    }
    @keyframes boot {
      from {
        transform: translateY(6px);
        opacity: 0;
      }
      to {
        transform: translateY(0);
        opacity: 1;
      }
    }
  </style>
</head>
<body>
  <div class="panel">
    <div class="topbar">
      <span>openclaude control center</span>
      <span class="boot-dot">online</span>
    </div>
    <div class="content">
      <div>
        <div class="title">READY FOR INPUT</div>
        <div class="sub">Terminal-oriented workflow with direct command access.</div>
      </div>

      <div class="terminal-box">
        <div class="terminal-row"><span class="prompt">$</span> openclaude --status</div>
        <div class="terminal-row">runtime: ${runtimeLabel}</div>
        <div class="terminal-row">shim: ${shimLabel}</div>
        <div class="terminal-row">command: ${status.executable}</div>
        <div class="terminal-row"><span class="prompt">$</span> <span class="cursor">awaiting command</span></div>
      </div>

      <div class="actions">
        <button class="btn primary" id="launch">Launch OpenClaude</button>
        <button class="btn" id="docs">Open Repository</button>
        <button class="btn" id="commands">Open Command Palette</button>
      </div>

      <div class="hint">
        Quick trigger: use <code>${status.shortcut}</code> and run OpenClaude commands from anywhere.
      </div>
    </div>
  </div>

  <script nonce="${nonce}">
    const vscode = acquireVsCodeApi();
    document.getElementById('launch').addEventListener('click', () => vscode.postMessage({ type: 'launch' }));
    document.getElementById('docs').addEventListener('click', () => vscode.postMessage({ type: 'docs' }));
    document.getElementById('commands').addEventListener('click', () => vscode.postMessage({ type: 'commands' }));
  </script>
</body>
</html>`;
  }
}

/**
 * @param {vscode.ExtensionContext} context
 */
function activate(context) {
  const startCommand = vscode.commands.registerCommand('openclaude.start', async () => {
    await launchOpenClaude();
  });

  const openDocsCommand = vscode.commands.registerCommand('openclaude.openDocs', async () => {
    await vscode.env.openExternal(vscode.Uri.parse(OPENCLAUDE_REPO_URL));
  });

  const openUiCommand = vscode.commands.registerCommand('openclaude.openControlCenter', async () => {
    await vscode.commands.executeCommand('workbench.view.extension.openclaude');
  });

  const provider = new OpenClaudeControlCenterProvider();
  const providerDisposable = vscode.window.registerWebviewViewProvider('openclaude.controlCenter', provider);

  context.subscriptions.push(startCommand, openDocsCommand, openUiCommand, providerDisposable);
}

function deactivate() {}

module.exports = {
  activate,
  deactivate,
};
