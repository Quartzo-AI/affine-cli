package canvaswrite

import (
	"strings"

	"affine-pp-cli/internal/config"
	"affine-pp-cli/internal/socketio"
)

func connect(cfg *config.Config, workspaceID string) (*socketio.Client, error) {
	bearer := strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return nil, err
	}
	if err := joinWorkspace(client, workspaceID); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}
