import * as vscode from 'vscode';
import { RPCEvent, AgentEventPayload } from './types';

/**
 * Gutter decoration types for file changes.
 */
const decorationTypes = {
  added: vscode.window.createTextEditorDecorationType({
    gutterIconSize: 'contain',
    before: {
      contentText: 'A',
      color: new vscode.ThemeColor('gitDecoration.addedResourceForeground'),
      fontWeight: 'bold',
    },
  }),
  modified: vscode.window.createTextEditorDecorationType({
    gutterIconSize: 'contain',
    before: {
      contentText: 'M',
      color: new vscode.ThemeColor('gitDecoration.modifiedResourceForeground'),
      fontWeight: 'bold',
    },
  }),
  deleted: vscode.window.createTextEditorDecorationType({
    gutterIconSize: 'contain',
    before: {
      contentText: 'D',
      color: new vscode.ThemeColor('gitDecoration.deletedResourceForeground'),
      fontWeight: 'bold',
    },
  }),
};

/**
 * Manages gutter decorations in the editor based on file_change events
 * from the Devon agent stream.
 */
export class IndicatorManager {
  private trackedFiles = new Map<string, 'added' | 'modified' | 'deleted'>();
  private activeEditor?: vscode.TextEditor;
  private editorChangeDisposable: vscode.Disposable;

  constructor() {
    this.editorChangeDisposable = vscode.window.onDidChangeActiveTextEditor(
      (editor) => {
        this.activeEditor = editor;
        this.updateDecorations();
      }
    );
    this.activeEditor = vscode.window.activeTextEditor;
  }

  /** Process an event from the event stream. */
  handleEvent(event: RPCEvent): void {
    const payload = event.payload as AgentEventPayload | undefined;

    if (event.type === 'file_change' && payload) {
      const filePath = payload.text || payload.args || '';
      const changeType = payload.result || 'modified';

      if (filePath) {
        this.trackedFiles.set(filePath, changeType as 'added' | 'modified' | 'deleted');
        this.updateDecorations();
      }
    }
  }

  /** Clear all tracked file indicators. */
  clear(): void {
    this.trackedFiles.clear();
    this.clearDecorations();
  }

  /** Dispose of decoration types and listeners. */
  dispose(): void {
    this.editorChangeDisposable.dispose();
    this.clearDecorations();
    this.trackedFiles.clear();

    for (const dt of Object.values(decorationTypes)) {
      dt.dispose();
    }
  }

  private updateDecorations(): void {
    if (!this.activeEditor) return;

    const doc = this.activeEditor.document;

    // Clear existing decorations
    this.clearDecorations();

    // Check if this file is tracked
    const changeType = this.trackedFiles.get(doc.fileName);
    if (!changeType) return;

    const decorationType = decorationTypes[changeType];
    if (!decorationType) return;

    // Apply decoration to all lines
    const range = new vscode.Range(0, 0, doc.lineCount - 1, 0);
    this.activeEditor.setDecorations(decorationType, [range]);
  }

  private clearDecorations(): void {
    for (const dt of Object.values(decorationTypes)) {
      if (this.activeEditor) {
        this.activeEditor.setDecorations(dt, []);
      }
    }
  }
}
