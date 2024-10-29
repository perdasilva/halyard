package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path"
	"strings"
)

//import (
//	"fmt"
//	tea "github.com/charmbracelet/bubbletea"
//	"halyard/internal/navigator"
//	"os"
//)
//
//func main() {
//	if _, err := tea.NewProgram(navigator.New(), tea.WithAltScreen()).Run(); err != nil {
//		fmt.Println("Error running program:", err)
//		os.Exit(1)
//	}
//}

const systemPrompt = `You are Halyard, a Kubernetes assistant specialized in managing OLM v1 resources: ClusterExtension and ClusterCatalog. 
You help the user explore, configure, and manage these resources in the cluster. Using a set of tools provided to you, you can:

    List and retrieve details of ClusterExtensions and ClusterCatalogs.
    Cache content from a ClusterCatalog to make it searchable.
    Search for packages within a ClusterCatalog.
    Assist in installing, updating, and deleting ClusterExtensions and ClusterCatalogs.
	Execute generated kubectl commands.

In each interaction, use these capabilities to efficiently address the user's requests. 
Always confirm actions that could change the state of resources (like installing, updating, or deleting) to avoid unintended modifications. 
When listing, retrieving, or searching resources, ensure the results are clear and relevant to the user's query.

If you don't know what to do, or if prompted for help, print out a helpful message introducing yourself and your capabilities.
Format your answers for the command-line terminal making use of ANSI colors and ASCII art to add flourish.`

func main() {

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = path.Join(home, ".kube", "config")
	}
	if stat, err := os.Stat(kubeconfig); err != nil || !stat.IsDir() {
		log.Fatalf("kubeconfig file %s does not exist", kubeconfig)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Initialize Ollama LLM with Langchaingo
	// llm, err := ollama.New(ollama.WithModel("llama3.2"))
	llm, err := openai.New()
	if err != nil {
		log.Fatal(err)
	}

	// Sending initial message to the model, with a list of available tools.
	ctx := context.Background()
	var messageHistory = []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
	}

	fmt.Println("Welcome to the Langchaingo Ollama chatbot! Type 'exit' to quit.")
	reader := bufio.NewReader(os.Stdin)
	for {
		// Get user input
		var userInput string
		fmt.Print("You: ")
		userInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		// Trim extra whitespace
		userInput = strings.TrimSpace(userInput)

		if userInput == "exit" {
			break
		}

		messageHistory = append(messageHistory, llms.TextParts(llms.ChatMessageTypeHuman, userInput))

		resp, err := llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
		if err != nil {
			log.Fatal(err)
		}
		messageHistory = updateMessageHistory(messageHistory, resp)

		// Execute tool calls requested by the model
		messageHistory = executeToolCalls(ctx, llm, messageHistory, resp)
		messageHistory = append(messageHistory, llms.TextParts(llms.ChatMessageTypeHuman, "Generate final response"))

		// Send query to the model again, this time with a history containing its
		// request to invoke a tool and our response to the tool call.
		resp, err = llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Bot: %s\n", resp.Choices[0].Content)
	}
}

// updateMessageHistory updates the message history with the assistant's
// response and requested tool calls.
func updateMessageHistory(messageHistory []llms.MessageContent, resp *llms.ContentResponse) []llms.MessageContent {
	respchoice := resp.Choices[0]

	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
	for _, tc := range respchoice.ToolCalls {
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	return append(messageHistory, assistantResponse)
}

// executeToolCalls executes the tool calls in the response and returns the
// updated message history.
func executeToolCalls(ctx context.Context, llm llms.Model, messageHistory []llms.MessageContent, resp *llms.ContentResponse) []llms.MessageContent {
	// fmt.Println("Executing", len(resp.Choices[0].ToolCalls), "tool calls")
	for _, toolCall := range resp.Choices[0].ToolCalls {
		switch toolCall.FunctionCall.Name {
		case "listClusterExtensions":
			response, err := listClusterExtensions()
			if err != nil {
				log.Fatal(err)
			}

			toolResponse := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: toolCall.ID,
						Name:       toolCall.FunctionCall.Name,
						Content:    response,
					},
				},
			}
			messageHistory = append(messageHistory, toolResponse)
		default:
			log.Fatalf("Unsupported tool: %s", toolCall.FunctionCall.Name)
		}
	}

	return messageHistory
}

func listClusterExtensions() (string, error) {
	response := map[string]interface{}{
		"list": []string{"argocd-operator", "prometheus-operator"},
	}

	b, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// availableTools simulates the tools/functions we're making available for
// the model.
var availableTools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "listClusterExtensions",
			Description: "List ClusterExtensions",
		},
	},
}

func showResponse(resp *llms.ContentResponse) string {
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return string(b)
}
