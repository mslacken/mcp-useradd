package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input struct for the Foo tool.
type FooInput struct {
	Message string `json:"message,omitempty" jsonschema:"a message for the Foo tool"`
}

// Output struct for the Foo tool.
type FooOutput struct {
	Response string `json:"response" jsonschema:"the response from the Foo tool"`
}

// Foo function implements the Foo tool.
func Foo(ctx context.Context, req *mcp.CallToolRequest, input FooInput) (
	*mcp.CallToolResult, FooOutput, error,
) {
	slog.Info("Foo tool called", "message", input.Message)
	return nil, FooOutput{Response: "Foo received your message: " + input.Message}, nil
}

func main() {
	listenAddr := flag.String("http", "", "address for http transport, defaults to stdio")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{Name: "useradd", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "Foo",
		Description: "A simple Foo tool",
	}, Foo)

	if *listenAddr == "" {
		// Run the server on the stdio transport.
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			slog.Error("Server failed", "error", err)
		}
	} else {
		// Create a streamable HTTP handler.
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)

		// Run the server on the HTTP transport.
		slog.Info("Server listening", "address", *listenAddr)
		if err := http.ListenAndServe(*listenAddr, handler); err != nil {
			slog.Error("Server failed", "error", err)
		}
	}
}
