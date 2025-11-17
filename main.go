package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// User struct represents a single user account.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	Comment  string `json:"comment"`
	Home     string `json:"home"`
	Shell    string `json:"shell"`
}

// Output struct for the ListUsers tool.
type ListUsersOutput struct {
	Users []User `json:"users" jsonschema:"the list of users on the system"`
}

// Input struct for the ListUsers tool.
type ListUsersInput struct{}

// ListUsers function implements the ListUsers tool.
func ListUsers(ctx context.Context, req *mcp.CallToolRequest, _ ListUsersInput) (
	*mcp.CallToolResult, ListUsersOutput, error,
) {
	slog.Info("ListUsers tool called")
	cmd := exec.Command("getent", "passwd")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, ListUsersOutput{}, err
	}
	var users []User
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 7 {
			continue
		}
		uid, _ := strconv.Atoi(parts[2])
		gid, _ := strconv.Atoi(parts[3])
		users = append(users, User{
			Username: parts[0],
			Password: parts[1],
			UID:      uid,
			GID:      gid,
			Comment:  parts[4],
			Home:     parts[5],
			Shell:    parts[6],
		})
	}
	return nil, ListUsersOutput{Users: users}, nil
}

func main() {
	listenAddr := flag.String("http", "", "address for http transport, defaults to stdio")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{Name: "useradd", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ListUsers",
		Description: "A tool to list the users on the system",
	}, ListUsers)

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
