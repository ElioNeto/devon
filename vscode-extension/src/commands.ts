import * as vscode from 'vscode';
import { RPCClient } from './client';
import { SessionInfo, StatusInfo } from './types';

/**
 * Register all Devon commands in the VS Code command palette.
 */
export function registerCommands(
  context: vscode.ExtensionContext,
  client: RPCClient
): void {
  context.subscriptions.push(
    vscode.commands.registerCommand('devon.sendPrompt', async () => {
      await handleSendPrompt(client);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('devon.interrupt', async () => {
      await handleInterrupt(client);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('devon.resumeSession', async () => {
      await handleResumeSession(client);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('devon.getStatus', async () => {
      await handleGetStatus(client);
    })
  );
}

async function handleSendPrompt(client: RPCClient): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  let defaultText = '';

  // Use selected text as default prompt
  if (editor && !editor.selection.isEmpty) {
    defaultText = editor.document.getText(editor.selection);
  }

  const prompt = await vscode.window.showInputBox({
    prompt: 'Enter prompt for Devon AI agent',
    placeHolder: 'e.g., refactor this function to use async/await',
    value: defaultText,
    ignoreFocusOut: true,
  });

  if (!prompt) return; // User cancelled

  try {
    const sessionInfo = await client.sendPrompt(prompt);
    vscode.window.showInformationMessage(
      `Devon: Prompt sent (session ${sessionInfo.id})`
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    vscode.window.showErrorMessage(`Devon: Failed to send prompt — ${msg}`);
  }
}

async function handleInterrupt(client: RPCClient): Promise<void> {
  try {
    await client.interrupt();
    vscode.window.showInformationMessage('Devon: Interrupt signal sent');
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    vscode.window.showErrorMessage(`Devon: Failed to interrupt — ${msg}`);
  }
}

async function handleResumeSession(client: RPCClient): Promise<void> {
  try {
    const sessions = await client.listSessions(20);
    if (sessions.length === 0) {
      vscode.window.showInformationMessage('Devon: No sessions available');
      return;
    }

    const items = sessions.map((s: SessionInfo) => ({
      label: `${s.id} — ${s.task || '(no task)'}`,
      description: `${s.status} | ${s.model || ''}`,
      detail: `${s.message_count} msgs, ${s.tool_call_count} tools`,
      session: s,
    }));

    const selected = await vscode.window.showQuickPick(items, {
      placeHolder: 'Select a session to resume',
      matchOnDescription: true,
      matchOnDetail: true,
    });

    if (!selected) return;

    // For resume we just show info - actual resume happens via sendPrompt with session context
    vscode.window.showInformationMessage(
      `Devon: Session ${selected.session.id} selected`
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    vscode.window.showErrorMessage(`Devon: Failed to list sessions — ${msg}`);
  }
}

async function handleGetStatus(client: RPCClient): Promise<void> {
  try {
    const status = await client.getStatus();
    if (status.running) {
      vscode.window.showInformationMessage(
        `Devon: Agent is running (session: ${status.session_id || 'N/A'}, model: ${status.model || 'N/A'})`
      );
    } else {
      vscode.window.showInformationMessage('Devon: Agent is not running');
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    vscode.window.showErrorMessage(`Devon: Failed to get status — ${msg}`);
  }
}
