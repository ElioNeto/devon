import * as net from 'net';
import * as path from 'path';
import {
  RPCRequest,
  RPCResponse,
  RPCEvent,
  AgentEventPayload,
  SendPromptParams,
  GetSessionParams,
  ListSessionsParams,
  SessionInfo,
  StatusInfo,
} from './types';

export interface ClientOptions {
  /** Path to the Unix socket. Defaults to <cwd>/.devon/rpc.sock */
  socketPath?: string;
  /** Reconnect delay in ms (default: 1000) */
  reconnectDelay?: number;
  /** Maximum reconnect attempts (default: 10, 0 = unlimited) */
  maxReconnectAttempts?: number;
}

export type EventHandler = (event: RPCEvent) => void;
export type StatusHandler = (connected: boolean) => void;

export class RPCClient {
  private socket: net.Socket | null = null;
  private buffer = '';
  private requestId = 1;
  private pending = new Map<number, {
    resolve: (resp: RPCResponse) => void;
    reject: (err: Error) => void;
    timer: NodeJS.Timeout;
  }>();
  private eventHandlers = new Set<EventHandler>();
  private statusHandlers = new Set<StatusHandler>();
  private connected = false;
  private reconnectAttempts = 0;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private disposed = false;

  public readonly options: Required<ClientOptions>;

  constructor(opts?: ClientOptions) {
    this.options = {
      socketPath: opts?.socketPath ?? path.join(process.cwd(), '.devon', 'rpc.sock'),
      reconnectDelay: opts?.reconnectDelay ?? 1000,
      maxReconnectAttempts: opts?.maxReconnectAttempts ?? 10,
    };
  }

  /** Connect to the Unix socket. */
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.disposed) {
        reject(new Error('Client is disposed'));
        return;
      }

      this.socket = new net.Socket();
      this.socket.connect(this.options.socketPath, () => {
        this.connected = true;
        this.reconnectAttempts = 0;
        this.notifyStatus(true);
        resolve();
      });

      this.socket.on('data', (data: Buffer) => {
        this.buffer += data.toString('utf-8');
        this.processBuffer();
      });

      this.socket.on('close', () => {
        this.connected = false;
        this.notifyStatus(false);
        this.rejectAllPending(new Error('Connection closed'));
        if (!this.disposed) {
          this.scheduleReconnect();
        }
      });

      this.socket.on('error', (err: Error) => {
        // If we're not connected yet, reject the connect promise
        if (!this.connected) {
          reject(err);
        } else {
          console.error('[devon-rpc] socket error:', err.message);
        }
      });

      // Set timeout for initial connection
      this.socket.on('connect', () => {
        // Already handled above
      });
    });
  }

  /** Disconnect from the socket. */
  disconnect(): void {
    this.disposed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.socket) {
      this.socket.destroy();
      this.socket = null;
    }
    this.connected = false;
    this.notifyStatus(false);
    this.rejectAllPending(new Error('Client disconnected'));
  }

  /** Send a JSON-RPC request and wait for response. */
  async call(method: string, params?: unknown): Promise<RPCResponse> {
    if (!this.connected || !this.socket) {
      throw new Error('Not connected to Devon RPC server');
    }

    const id = this.requestId++;
    const request: RPCRequest = {
      jsonrpc: '2.0',
      id,
      method,
      params,
    };

    return new Promise<RPCResponse>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`RPC call '${method}' timed out`));
      }, 30000);

      this.pending.set(id, { resolve, reject, timer });

      try {
        this.socket!.write(JSON.stringify(request) + '\n');
      } catch (err) {
        this.pending.delete(id);
        clearTimeout(timer);
        reject(err instanceof Error ? err : new Error(String(err)));
      }
    });
  }

  /** Send a prompt to the agent. */
  async sendPrompt(prompt: string, mode?: string): Promise<SessionInfo> {
    const params: SendPromptParams = { prompt, mode };
    const resp = await this.call('sendPrompt', params);
    if (resp.error) {
      throw new Error(`sendPrompt failed: ${resp.error.message}`);
    }
    return resp.result as SessionInfo;
  }

  /** Get session details. */
  async getSession(id: string): Promise<SessionInfo> {
    const params: GetSessionParams = { id };
    const resp = await this.call('getSession', params);
    if (resp.error) {
      throw new Error(`getSession failed: ${resp.error.message}`);
    }
    return resp.result as SessionInfo;
  }

  /** List recent sessions. */
  async listSessions(limit?: number): Promise<SessionInfo[]> {
    const params: ListSessionsParams = { limit };
    const resp = await this.call('listSessions', params);
    if (resp.error) {
      throw new Error(`listSessions failed: ${resp.error.message}`);
    }
    return resp.result as SessionInfo[];
  }

  /** Interrupt the current agent execution. */
  async interrupt(): Promise<void> {
    const resp = await this.call('interrupt');
    if (resp.error) {
      throw new Error(`interrupt failed: ${resp.error.message}`);
    }
  }

  /** Get agent status. */
  async getStatus(): Promise<StatusInfo> {
    const resp = await this.call('getStatus');
    if (resp.error) {
      throw new Error(`getStatus failed: ${resp.error.message}`);
    }
    return resp.result as StatusInfo;
  }

  // Event handling

  /** Register an event handler for broadcast events. */
  onEvent(handler: EventHandler): () => void {
    this.eventHandlers.add(handler);
    return () => this.eventHandlers.delete(handler);
  }

  /** Register a connection status change handler. */
  onStatusChange(handler: StatusHandler): () => void {
    this.statusHandlers.add(handler);
    return () => this.statusHandlers.delete(handler);
  }

  // Private methods

  private processBuffer(): void {
    const lines = this.buffer.split('\n');
    // Keep the last potentially incomplete line in the buffer
    this.buffer = lines.pop() || '';

    for (const line of lines) {
      if (!line.trim()) continue;

      try {
        const parsed = JSON.parse(line);

        // Check if it's a response (has id)
        if (parsed.id !== undefined) {
          this.handleResponse(parsed as RPCResponse);
        } else if (parsed.type) {
          // It's a broadcast event
          this.handleEvent(parsed as RPCEvent);
        }
      } catch (err) {
        console.warn('[devon-rpc] failed to parse message:', (err as Error).message);
      }
    }
  }

  private handleResponse(resp: RPCResponse): void {
    const id = resp.id;
    if (id === undefined) return;

    const pending = this.pending.get(id);
    if (!pending) {
      console.warn('[devon-rpc] unexpected response id:', id);
      return;
    }

    clearTimeout(pending.timer);
    this.pending.delete(id);
    pending.resolve(resp);
  }

  private handleEvent(event: RPCEvent): void {
    for (const handler of this.eventHandlers) {
      try {
        handler(event);
      } catch (err) {
        console.error('[devon-rpc] event handler error:', err);
      }
    }
  }

  private scheduleReconnect(): void {
    if (this.disposed) return;
    if (this.options.maxReconnectAttempts > 0 &&
        this.reconnectAttempts >= this.options.maxReconnectAttempts) {
      console.error('[devon-rpc] max reconnect attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = this.options.reconnectDelay * Math.min(this.reconnectAttempts, 5);
    console.log(`[devon-rpc] reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

    this.reconnectTimer = setTimeout(async () => {
      if (this.disposed) return;
      try {
        await this.connect();
      } catch {
        this.scheduleReconnect();
      }
    }, delay);
  }

  private rejectAllPending(err: Error): void {
    for (const [id, pending] of this.pending) {
      clearTimeout(pending.timer);
      pending.reject(err);
    }
    this.pending.clear();
  }

  private notifyStatus(connected: boolean): void {
    this.connected = connected;
    for (const handler of this.statusHandlers) {
      try {
        handler(connected);
      } catch (err) {
        console.error('[devon-rpc] status handler error:', err);
      }
    }
  }

  get isConnected(): boolean {
    return this.connected;
  }
}
