package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	llmTools "github.com/tmc/langchaingo/tools"
	"os/exec"
	"strings"
)

type kubectlTool struct {
}

func (t *kubectlTool) Name() string {
	return "kubectl_tool"
}

func (t *kubectlTool) Description() string {
	return `Execute kubectl commands against the cluster. Arguments should be in the format {"args": <arguments string>, "manifest": <optional kubernetes resource manifest>}.
Examples:
1. Create a new namespace called example-namespace:
	{
		"args": "apply -f -",
		"manifest": "apiVersion: v1
kind: Namespace
metadata:
  name: example-namespace"
	}

2. Run kubectl get namespaces:
{
	"args": "get namespaces"
}
`
}

func (t *kubectlTool) Call(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Args     string `json:"args"`
		Manifest string `json:"manifest"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf("error parsing parameters: %v", err), nil
	}

	split := strings.Split(args.Args, " ")

	cmd := exec.Command("kubectl", split...)

	// If manifest data is provided, use it as input (e.g., for kubectl apply -f -)
	if args.Manifest != "" {
		cmd.Stdin = bytes.NewReader([]byte(args.Manifest))
	}

	// Capture the command's output and error
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Sprintf("kubectl command failed: %v, %s", err, stderr.String()), nil
	}

	return out.String(), nil
}

func NewKubectlTool() llmTools.Tool {
	return &kubectlTool{}
}
