import * as assert from 'assert';
import * as net from 'net';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs';
import { RPCClient } from '../../client';
import { RPCResponse, SessionInfo, StatusInfo } from '../../types';

/**
 * Helper: start a mock Unix socket server that responds to JSON-RPC requests.
 */
function startMockServer(
  socketPath: string,
  handler?: (req: string) => string
): Promise<net.Server> {
  return new Promise((resolve, reject) => {
    // Remove stale socket
    try {
      fs.unlinkSync(socketPath);
    } catch {
      // OK if doesn't exist
    }

    const dir = path.dirname(socketPath);
    fs.mkdirSync(dir, { recursive: true });

    const server = net.createServer((conn) => {
      let buffer = '';
      conn.on('data', (data: Buffer) => {
        buffer += data.toString('utf-8');
        const lines = buffer.split('\n');

        for (let i = 0; i < lines.length - 1; i++) {
          const line = lines[i].trim();
          if (!line) continue;

          let response: string;
          if (handler) {
            response = handler(line);
          } else {
            // Default echo handler
            response = JSON.stringify({
              jsonrpc: '2.0',
              id: 1,
              result: { status: 'ok' },
            });
          }
          conn.write(response + '\n');
        }
        buffer = lines[lines.length - 1] || '';
      });
    });

    server.on('error', reject);

    server.listen(socketPath, () => {
      resolve(server);
    });
  });
}

suite('RPCClient', () => {
  let mockServer: net.Server;
  let socketPath: string;
  let client: RPCClient;

  const tmpDir = path.join(os.tmpdir(), 'devon-test-' + Date.now());

  setup(async () => {
    socketPath = path.join(tmpDir, 'rpc.sock');
    fs.mkdirSync(path.dirname(socketPath), { recursive: true });

    mockServer = await startMockServer(socketPath);
    client = new RPCClient({ socketPath });
  });

  teardown(() => {
    try {
      client.disconnect();
      mockServer.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    } catch {
      // ignore cleanup errors
    }
  });

  test('connect and disconnect', async () => {
    assert.strictEqual(client.isConnected, false);

    await client.connect();
    assert.strictEqual(client.isConnected, true);

    client.disconnect();
    assert.strictEqual(client.isConnected, false);
  });

  test('send request and receive response', async () => {
    // Override server to respond to specific method
    mockServer.close();

    mockServer = await startMockServer(socketPath, (req) => {
      const parsed = JSON.parse(req);
      assert.strictEqual(parsed.method, 'getStatus');
      return JSON.stringify({
        jsonrpc: '2.0',
        id: parsed.id,
        result: { running: true, session_id: 'test-123' },
      });
    });

    await client.connect();
    const status = await client.getStatus();
    assert.ok(status.running);
    assert.strictEqual(status.session_id, 'test-123');
  });

  test('sendPrompt method', async () => {
    mockServer.close();
    mockServer = await startMockServer(socketPath, (req) => {
      const parsed = JSON.parse(req);
      assert.strictEqual(parsed.method, 'sendPrompt');
      assert.strictEqual(parsed.params.prompt, 'test prompt');
      return JSON.stringify({
        jsonrpc: '2.0',
        id: parsed.id,
        result: {
          id: 'session-1',
          status: 'active',
          message_count: 0,
          tool_call_count: 0,
        },
      });
    });

    await client.connect();
    const info = await client.sendPrompt('test prompt');
    assert.strictEqual(info.id, 'session-1');
    assert.strictEqual(info.status, 'active');
  });

  test('interrupt method', async () => {
    mockServer.close();
    mockServer = await startMockServer(socketPath, (req) => {
      const parsed = JSON.parse(req);
      assert.strictEqual(parsed.method, 'interrupt');
      return JSON.stringify({
        jsonrpc: '2.0',
        id: parsed.id,
        result: { status: 'interrupted' },
      });
    });

    await client.connect();
    await client.interrupt(); // should not throw
  });

  test('listSessions method', async () => {
    mockServer.close();
    mockServer = await startMockServer(socketPath, (req) => {
      const parsed = JSON.parse(req);
      assert.strictEqual(parsed.method, 'listSessions');
      return JSON.stringify({
        jsonrpc: '2.0',
        id: parsed.id,
        result: [
          { id: 's1', status: 'active', message_count: 5, tool_call_count: 2 },
          { id: 's2', status: 'completed', message_count: 10, tool_call_count: 3 },
        ],
      });
    });

    await client.connect();
    const sessions = await client.listSessions();
    assert.strictEqual(sessions.length, 2);
    assert.strictEqual(sessions[0].id, 's1');
  });

  test('error response handling', async () => {
    mockServer.close();
    mockServer = await startMockServer(socketPath, (req) => {
      return JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        error: { code: -32601, message: 'Method not found' },
      });
    });

    await client.connect();

    try {
      await client.getStatus();
      assert.fail('Expected error');
    } catch (err) {
      assert.ok((err as Error).message.includes('Method not found'));
    }
  });

  test('broadcast event handling', async () => {
    mockServer.close();

    const receivedEvents: any[] = [];
    mockServer = await startMockServer(socketPath, (req) => {
      // Return a normal response
      return JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        result: { running: true },
      });
    });

    await client.connect();

    client.onEvent((event) => {
      receivedEvents.push(event);
    });

    // Simulate a broadcast by writing to the socket directly
    const broadcastMsg = JSON.stringify({
      type: 'tool_start',
      payload: { type: 'tool_start', tool: 'read_file' },
    });
    const conn = (client as any).socket as net.Socket;
    conn.emit('data', Buffer.from(broadcastMsg + '\n', 'utf-8'));

    // Wait for async processing
    await new Promise((resolve) => setTimeout(resolve, 100));

    assert.strictEqual(receivedEvents.length, 1);
    assert.strictEqual(receivedEvents[0].type, 'tool_start');
  });

  test('connection status change handler', async () => {
    const statusChanges: boolean[] = [];

    await client.connect();

    client.onStatusChange((connected) => {
      statusChanges.push(connected);
    });

    // Simulate disconnect via socket close
    const conn = (client as any).socket as net.Socket;
    conn.emit('close');

    // Wait for async processing
    await new Promise((resolve) => setTimeout(resolve, 100));

    assert.ok(statusChanges.includes(false));
  });

  test('getSession method', async () => {
    mockServer.close();
    mockServer = await startMockServer(socketPath, (req) => {
      const parsed = JSON.parse(req);
      assert.strictEqual(parsed.method, 'getSession');
      assert.strictEqual(parsed.params.id, 'session-1');
      return JSON.stringify({
        jsonrpc: '2.0',
        id: parsed.id,
        result: {
          id: 'session-1',
          task: 'test task',
          status: 'active',
          message_count: 5,
          tool_call_count: 2,
          total_cost: 0.05,
          duration_ms: 1000,
        },
      });
    });

    await client.connect();
    const info = await client.getSession('session-1');
    assert.strictEqual(info.id, 'session-1');
    assert.strictEqual(info.task, 'test task');
    assert.strictEqual(info.total_cost, 0.05);
  });
});
