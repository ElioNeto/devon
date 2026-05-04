// Package headless implements an HTTP/SSE server that exposes the Devon agent
// to CI/CD pipelines and external HTTP clients without requiring a TUI.
//
// # Purpose
//
// The headless server allows automated systems (CI/CD, scripts, external tools)
// to send prompts to the Devon agent, stream responses via Server-Sent Events (SSE),
// handle blocking confirmation dialogs, and inspect agent status — all over plain HTTP.
//
// # Public API
//
// The server exposes four endpoints:
//
//	POST /api/prompt   — Submit a prompt and stream agent events via SSE.
//	POST /api/respond  — Respond to an action_required (confirmation) request.
//	GET  /api/status   — Query whether an agent is currently running and return
//	                     metadata (session ID, model, task type).
//	GET  /api/health   — Health check returning {"status":"ok"}.
//
// # Transport
//
// All communication uses HTTP with JSON request/response bodies.
// The /api/prompt endpoint uses Server-Sent Events (SSE) for streaming agent
// output. This avoids the need for WebSockets or gRPC and is compatible with
// standard HTTP clients, proxies, and tools like curl.
//
// # Integration with the agent
//
// When a POST /api/prompt request arrives, the server:
//  1. Validates and parses the JSON body (PromptRequest).
//  2. Creates (or reuses) a session, optionally resuming from a session_id.
//  3. Launches an agent goroutine that processes the prompt.
//  4. Streams agent events (text_delta, tool_start, tool_done, turn_done,
//     action_required, error, etc.) as SSE data frames.
//  5. On action_required, the server blocks and waits for a /api/respond call
//     from the client before continuing.
//
// # Example usage
//
//	// Start the server
//	srv, err := headless.NewServer("127.0.0.1", 9876)
//	if err != nil { ... }
//	headless.RegisterHandlers(srv, cfg, registry, router)
//	srv.Listen()
//	go srv.Serve()
//
//	// Client: send a prompt
//	curl -X POST http://127.0.0.1:9876/api/prompt \
//	  -H "Content-Type: application/json" \
//	  -d '{"prompt":"Refactor the main function","mode":"auto"}'
//
//	# Response (SSE stream):
//	# data: {"type":"text_delta","payload":{"text":"I'll refactor main..."}}
//	# data: {"type":"tool_start","payload":{"tool":"read","args":"..."}}
//	# data: {"type":"tool_done","payload":{"tool":"read","result":"..."}}
//	# data: {"type":"turn_done","payload":{}}
//
//	// Client: respond to action_required
//	curl -X POST http://127.0.0.1:9876/api/respond \
//	  -H "Content-Type: application/json" \
//	  -d '{"request_id":"req-1","approved":true}'
package headless
