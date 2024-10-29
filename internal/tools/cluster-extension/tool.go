package cluster_extension

import (
	"context"
	"encoding/json"
	llmTools "github.com/tmc/langchaingo/tools"
)

type listClusterExtensionsTool struct{}

func (t *listClusterExtensionsTool) Name() string {
	return "listClusterExtensions"
}

func (t *listClusterExtensionsTool) Description() string {
	return "List ClusterExtensions on the cluster"
}

func (t *listClusterExtensionsTool) Call(ctx context.Context, _ string) (string, error) {
	response := map[string]interface{}{
		"list": []string{"argocd-operator"},
	}
	b, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func NewListTool() llmTools.Tool {
	return &listClusterExtensionsTool{}
}
