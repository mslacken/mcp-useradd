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
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	UID          int      `json:"uid"`
	GID          int      `json:"gid"`
	Comment      string   `json:"comment"`
	Home         string   `json:"home"`
	Shell        string   `json:"shell"`
	IsSystemUser bool     `json:"is_system_user"`
	Groups       []string `json:"groups"`
}

// Group struct represents a single group.
type Group struct {
	Name     string   `json:"name"`
	Password string   `json:"password"`
	GID      int      `json:"gid"`
	Members  []string `json:"members"`
}

// Output struct for the ListUsers tool.
type ListUsersOutput struct {
	Users  []User  `json:"users" jsonschema:"the list of users on the system"`
	Groups []Group `json:"groups" jsonschema:"the list of groups on the system"`
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
			Username:     parts[0],
			Password:     parts[1],
			UID:          uid,
			GID:          gid,
			Comment:      parts[4],
			Home:         parts[5],
			Shell:        parts[6],
			IsSystemUser: gid < 1000,
			Groups:       []string{},
		})
	}

	cmd = exec.Command("getent", "group")
	var groupOut bytes.Buffer
	cmd.Stdout = &groupOut
	err = cmd.Run()
	if err != nil {
		return nil, ListUsersOutput{}, err
	}
	var groups []Group
	groupScanner := bufio.NewScanner(&groupOut)
	for groupScanner.Scan() {
		line := groupScanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 4 {
			continue
		}
		gid, _ := strconv.Atoi(parts[2])
		members := strings.Split(parts[3], ",")
		groups = append(groups, Group{
			Name:     parts[0],
			Password: parts[1],
			GID:      gid,
			Members:  members,
		})
		groupName := parts[0]
		for _, member := range members {
			for i, user := range users {
				if user.Username == member {
					users[i].Groups = append(users[i].Groups, groupName)
				}
			}
		}
	}

	return nil, ListUsersOutput{Users: users, Groups: groups}, nil
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
