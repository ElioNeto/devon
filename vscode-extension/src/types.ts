// JSON-RPC 2.0 types matching internal/rpc/types.go

export interface RPCRequest {
  jsonrpc: string;
  id?: number;
  method: string;
  params?: unknown;
}

export interface RPCResponse {
  jsonrpc: string;
  id?: number;
  result?: unknown;
  error?: RPCError;
}

export interface RPCError {
  code: number;
  message: string;
  data?: unknown;
}

// Standard JSON-RPC 2.0 error codes
export const ErrParse = -32700;
export const ErrInvalidRequest = -32600;
export const ErrMethodNotFound = -32601;
export const ErrInvalidParams = -32602;
export const ErrInternal = -32603;
export const ErrServer = -32000;

// Event from server broadcast
export interface RPCEvent {
  type: string;
  payload?: unknown;
}

// Session info
export interface SessionInfo {
  id: string;
  task?: string;
  model?: string;
  status: string;
  message_count: number;
  tool_call_count: number;
  total_cost?: number;
  duration_ms?: number;
}

// Status info
export interface StatusInfo {
  running: boolean;
  session_id?: string;
  model?: string;
  task_type?: string;
}

// Agent event types
export interface AgentEventPayload {
  type: string;
  text?: string;
  tool?: string;
  args?: string;
  result?: string;
  error?: string;
}

// Method parameter types
export interface SendPromptParams {
  prompt: string;
  mode?: string;
}

export interface GetSessionParams {
  id: string;
}

export interface ListSessionsParams {
  limit?: number;
}
