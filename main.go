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

// Input struct for the AddUser tool.
type AddUserInput struct {
	Username     string   `json:"username" jsonschema:"the username of the new account"`
	BaseDir      string   `json:"base_dir,omitempty" jsonschema:"the base directory for the home directory of the new account"`
	Comment      string   `json:"comment,omitempty" jsonschema:"the GECOS field of the new account"`
	HomeDir      string   `json:"home_dir,omitempty" jsonschema:"the home directory of the new account"`
	ExpireDate   string   `json:"expire_date,omitempty" jsonschema:"the expiration date of the new account"`
	Inactive     int      `json:"inactive,omitempty" jsonschema:"the password inactivity period of the new account"`
	Gid          string   `json:"gid,omitempty" jsonschema:"the name or ID of the primary group of the new account"`
	Groups       []string `json:"groups,omitempty" jsonschema:"the list of supplementary groups of the new account"`
	SkelDir      string   `json:"skel_dir,omitempty" jsonschema:"the alternative skeleton directory"`
	CreateHome   bool     `json:"create_home,omitempty" jsonschema:"create the user's home directory"`
	NoCreateHome bool     `json:"no_create_home,omitempty" jsonschema:"do not create the user's home directory"`
	NoUserGroup  bool     `json:"no_user_group,omitempty" jsonschema:"do not create a group with the same name as the user"`
	NonUnique    bool     `json:"non_unique,omitempty" jsonschema:"allow to create users with duplicate (non-unique) UID"`
	Password     string   `json:"password,omitempty" jsonschema:"the encrypted password of the new account"`
	System       bool     `json:"system,omitempty" jsonschema:"create a system account"`
	Shell        string   `json:"shell,omitempty" jsonschema:"the login shell of the new account"`
	Uid          int      `json:"uid,omitempty" jsonschema:"the user ID of the new account"`
	UserGroup    bool     `json:"user_group,omitempty" jsonschema:"create a group with the same name as the user"`
	SelinuxUser  string   `json:"selinux_user,omitempty" jsonschema:"the specific SEUSER for the SELinux user mapping"`
	SelinuxRange string   `json:"selinux_range,omitempty" jsonschema:"the specific MLS range for the SELinux user mapping"`
}

// Output struct for the AddUser tool.
type AddUserOutput struct {
	Success bool   `json:"success" jsonschema:"whether the user was added successfully"`
	Message string `json:"message" jsonschema:"a message indicating the result of the operation"`
}

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
	Groups []Group `json:"groups,omitempty" jsonschema:"the list of groups on the system"`
}

// Input struct for the ListUsers tool.
type ListUsersInput struct {
	Username string `json:"username,omitempty" jsonschema:"the optional username to filter by"`
}

// ListUsers function implements the ListUsers tool.
func ListUsers(ctx context.Context, req *mcp.CallToolRequest, input ListUsersInput) (
	*mcp.CallToolResult, ListUsersOutput, error,
) {
	slog.Info("ListUsers tool called")

	users, err := getUsers(input.Username)
	if err != nil {
		return nil, ListUsersOutput{}, err
	}

	if input.Username != "" {
		return nil, ListUsersOutput{Users: users}, nil
	}

	groups, err := getGroups()
	if err != nil {
		return nil, ListUsersOutput{}, err
	}

	for _, group := range groups {
		for _, member := range group.Members {
			for i, user := range users {
				if user.Username == member {
					users[i].Groups = append(users[i].Groups, group.Name)
				}
			}
		}
	}

	return nil, ListUsersOutput{Users: users, Groups: groups}, nil
}

func getUsers(username string) ([]User, error) {
	args := []string{"passwd"}
	if username != "" {
		args = append(args, username)
	}
	cmd := exec.Command("getent", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
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
	if username != "" && len(users) > 0 {
		groups, err := getUserGroups(username)
		if err == nil {
			users[0].Groups = groups
		}
	}
	return users, nil
}

func getUserGroups(username string) ([]string, error) {
	cmd := exec.Command("getent", "group", username)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	var groups []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) > 0 {
			groups = append(groups, parts[0])
		}
	}
	return groups, nil
}

func getGroups() ([]Group, error) {
	cmd := exec.Command("getent", "group")
	var groupOut bytes.Buffer
	cmd.Stdout = &groupOut
	err := cmd.Run()
	if err != nil {
		return nil, err
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
	}
	return groups, nil
}

func AddUser(ctx context.Context, req *mcp.CallToolRequest, input AddUserInput) (
	*mcp.CallToolResult, AddUserOutput, error,
) {
	slog.Info("AddUser tool called")
	args := []string{}
	if input.BaseDir != "" {
		args = append(args, "-b", input.BaseDir)
	}
	if input.Comment != "" {
		args = append(args, "-c", input.Comment)
	}
	if input.HomeDir != "" {
		args = append(args, "-d", input.HomeDir)
	}
	if input.ExpireDate != "" {
		args = append(args, "-e", input.ExpireDate)
	}
	if input.Inactive != 0 {
		args = append(args, "-f", strconv.Itoa(input.Inactive))
	}
	if input.Gid != "" {
		args = append(args, "-g", input.Gid)
	}
	if len(input.Groups) > 0 {
		args = append(args, "-G", strings.Join(input.Groups, ","))
	}
	if input.SkelDir != "" {
		args = append(args, "-k", input.SkelDir)
	}
	if input.CreateHome {
		args = append(args, "-m")
	}
	if input.NoCreateHome {
		args = append(args, "-M")
	}
	if input.NoUserGroup {
		args = append(args, "-N")
	}
	if input.NonUnique {
		args = append(args, "-o")
	}
	if input.Password != "" {
		args = append(args, "-p", input.Password)
	}
	if input.System {
		args = append(args, "-r")
	}
	if input.Shell != "" {
		args = append(args, "-s", input.Shell)
	}
	if input.Uid != 0 {
		args = append(args, "-u", strconv.Itoa(input.Uid))
	}
	if input.UserGroup {
		args = append(args, "-U")
	}
	if input.SelinuxUser != "" {
		args = append(args, "-Z", input.SelinuxUser)
	}
	args = append(args, input.Username)

	cmd := exec.Command("useradd", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return nil, AddUserOutput{Success: false, Message: out.String()}, err
	}
	return nil, AddUserOutput{Success: true, Message: out.String()}, nil
}

func main() {
	listenAddr := flag.String("http", "", "address for http transport, defaults to stdio")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{Name: "useradd", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ListUsers",
		Description: "A tool to list the users on the system",
	}, ListUsers)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "AddUser",
		Description: "A tool to add a new user to the system",
	}, AddUser)

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
