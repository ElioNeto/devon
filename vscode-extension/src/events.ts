import * as vscode from 'vscode';
import { RPCClient } from './client';
import { RPCEvent, AgentEventPayload } from './types';

/**
 * Manages event streaming from the RPC client and dispatches events
 * to VS Code UI components.
 */
export class EventManager {
  private client: RPCClient;
  private disposeHandlers: (() => void)[] = [];
  private statusBarItem: vscode.StatusBarItem;
  private outputChannel: vscode.OutputChannel;

  constructor(client: RPCClient) {
    this.client = client;
    this.statusBarItem = vscode.window.createStatusBarItem(
      vscode.StatusBarAlignment.Left,
      100
    );
    this.statusBarItem.text = '$(sync~spin) Devon';
    this.statusBarItem.tooltip = 'Devon AI Agent';
    this.statusBarItem.command = 'devon.getStatus';
    this.statusBarItem.show();

    this.outputChannel = vscode.window.createOutputChannel('Devon Agent');
  }

  /** Start listening for events. */
  start(): void {
    const unsubEvent = this.client.onEvent((event: RPCEvent) => {
      this.handleEvent(event);
    });
    this.disposeHandlers.push(unsubEvent);

    const unsubStatus = this.client.onStatusChange((connected: boolean) => {
      this.handleConnectionChange(connected);
    });
    this.disposeHandlers.push(unsubStatus);
  }

  /** Stop listening and clean up. */
  dispose(): void {
    for (const dispose of this.disposeHandlers) {
      dispose();
    }
    this.disposeHandlers = [];
    this.statusBarItem.dispose();
    this.outputChannel.dispose();
  }

  private handleEvent(event: RPCEvent): void {
    const payload = event.payload as AgentEventPayload | undefined;

    switch (event.type) {
      case 'text':
        if (payload?.text) {
          this.outputChannel.append(payload.text);
        }
        break;

      case 'tool_start':
        if (payload?.tool) {
          this.outputChannel.appendLine(`\n[tool] ${payload.tool}...`);
          this.statusBarItem.text = `$(tools) Devon: ${payload.tool}`;
        }
        break;

      case 'tool_done':
        if (payload?.tool) {
          this.outputChannel.appendLine(`[tool] ${payload.tool} ✓`);
          this.statusBarItem.text = '$(sync~spin) Devon';
        }
        break;

      case 'tool_error':
        if (payload?.tool && payload?.error) {
          this.outputChannel.appendLine(`[tool] ${payload.tool} ✗ ${payload.error}`);
          this.statusBarItem.text = '$(error) Devon';
        }
        break;

      case 'error':
        if (payload?.error) {
          this.outputChannel.appendLine(`\n[error] ${payload.error}`);
          this.statusBarItem.text = '$(error) Devon';
          vscode.window.showErrorMessage(`Devon: ${payload.error}`);
        }
        break;

      case 'turn_done':
        this.outputChannel.appendLine('\n--- Turn complete ---');
        this.statusBarItem.text = '$(check) Devon';
        break;

      case 'system':
        if (payload?.text) {
          this.outputChannel.appendLine(`\n[system] ${payload.text}`);
        }
        break;

      default:
        this.outputChannel.appendLine(`\n[event] ${event.type}`);
        break;
    }
  }

  private handleConnectionChange(connected: boolean): void {
    if (connected) {
      this.statusBarItem.text = '$(plug) Devon';
      this.statusBarItem.tooltip = 'Devon AI Agent — Connected';
      this.outputChannel.appendLine('[Devon] Connected to agent');
    } else {
      this.statusBarItem.text = '$(plug) Devon';
      this.statusBarItem.tooltip = 'Devon AI Agent — Disconnected';
      this.statusBarItem.backgroundColor = new vscode.ThemeColor(
        'statusBarItem.warningBackground'
      );
      this.outputChannel.appendLine('[Devon] Disconnected from agent');
    }
  }
}
