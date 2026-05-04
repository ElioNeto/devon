import * as path from 'path';
import * as vscode from 'vscode';
import { RPCClient } from './client';
import { registerCommands } from './commands';
import { EventManager } from './events';
import { IndicatorManager } from './indicators';

let client: RPCClient | undefined;
let eventManager: EventManager | undefined;
let indicatorManager: IndicatorManager | undefined;

/**
 * Activate the Devon VS Code extension.
 *
 * Connects to the Devon RPC server via Unix socket, registers
 * commands, starts event streaming, and creates status bar UI.
 */
export function activate(context: vscode.ExtensionContext): void {
  console.log('[devon] activating extension');

  // Derive socket path from workspace root (matches Go server's DefaultSocketPath resolution)
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri?.fsPath;
  const socketPath = workspaceRoot
    ? path.join(workspaceRoot, '.devon', 'rpc.sock')
    : undefined;

  // Create RPC client
  client = new RPCClient({ socketPath });

  // Create event manager (handles status bar + output channel)
  eventManager = new EventManager(client);
  context.subscriptions.push({
    dispose: () => eventManager?.dispose(),
  });

  // Create indicator manager (gutter decorations)
  indicatorManager = new IndicatorManager();
  context.subscriptions.push({
    dispose: () => indicatorManager?.dispose(),
  });

  // Register commands
  registerCommands(context, client);

  // Connect to the RPC server
  connectWithRetry(client, context);
}

/**
 * Attempt to connect to the RPC server, retrying if it's not ready yet.
 */
async function connectWithRetry(
  rpcClient: RPCClient,
  context: vscode.ExtensionContext
): Promise<void> {
  const maxRetries = 5;
  const retryDelay = 2000; // 2 seconds

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      await rpcClient.connect();
      console.log('[devon] connected to RPC server');

      // Start event streaming
      eventManager?.start();

      // Forward events to indicator manager
      rpcClient.onEvent((event) => {
        indicatorManager?.handleEvent(event);
      });

      return;
    } catch (err) {
      if (attempt < maxRetries) {
        console.log(
          `[devon] connection attempt ${attempt}/${maxRetries} failed, retrying in ${retryDelay}ms...`
        );
        await sleep(retryDelay);
      } else {
        const msg = err instanceof Error ? err.message : String(err);
        console.error(`[devon] failed to connect after ${maxRetries} attempts: ${msg}`);

        vscode.window.showWarningMessage(
          `Devon: Could not connect to agent. Make sure 'devon rpc' is running. (${msg})`
        );
      }
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Deactivate the Devon VS Code extension.
 */
export function deactivate(): void {
  console.log('[devon] deactivating extension');

  if (eventManager) {
    eventManager.dispose();
    eventManager = undefined;
  }

  if (indicatorManager) {
    indicatorManager.dispose();
    indicatorManager = undefined;
  }

  if (client) {
    client.disconnect();
    client = undefined;
  }
}
