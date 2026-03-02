# Ollama Package

This package provides types and a client for interacting with the Ollama API, specifically for chat and tool execution.

## Key Types

- `ChatRequest`: Request body for Ollama chat.
- `Message`: Chat message with role, content, and tool calls.
- `StreamResponse`: Parsed stream updates.

## Key Components

- `Client`: Handles HTTP requests to the Ollama server.
- `StreamWorker`: Asynchronous worker for reading NDJSON stream updates.
- `ParseToolArguments`: Safe parsing of tool call arguments, handling both raw objects and stringified JSON.
