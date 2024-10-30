package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
	"halyard/internal/cache"
	"halyard/internal/tools/clustercatalog"
	"halyard/internal/tools/clusterextension"
	"halyard/internal/tools/k8s"
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

const promptPrefix = `You are Halyard, a helpful Kubernetes CLI assistant specialized in managing OLM v1 resources: ClusterExtension and ClusterCatalog. You have access to the following tools:

{{.tool_descriptions}}`

const promptFormat = `From now on strictly use the following format:

Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [ {{.tool_names}} ]
Action Input: the input to the action
Observation: the result of the action
... (this Thought/Action/Action Input/Observation can repeat N times)
Thought: I now know the final answer
Final Answer: a succinct final answer to the original input question, which is fit for a CLI application, does not use markdown format, and relies on ANSI colors for styling and emoticons to provide a nice user experience.`

const promptSuffix = `Begin!

Question: {{.input}}
Thought:{{.agent_scratchpad}}`

func main() {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = path.Join(home, ".kube", "config")
	}
	if stat, err := os.Stat(kubeconfig); err != nil || stat.IsDir() {
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
	llm, err := openai.New(openai.WithModel("gpt-4o"))
	if err != nil {
		log.Fatal(err)
	}

	packageCache := cache.NewPackageCache()

	// Sending initial message to the model, with a list of available tools.
	agentTools := []tools.Tool{
		clusterextension.NewListTool(dynClient),
		clusterextension.NewGetTool(dynClient),
		clusterextension.NewGenerateTool(llm),
		clustercatalog.NewListTool(dynClient),
		clustercatalog.NewGetTool(dynClient),
		clustercatalog.NewCacheTool(config, packageCache),
		clustercatalog.NewSearchCatalogTool(packageCache),
		k8s.NewKubectlTool(),
	}

	agent := agents.NewOneShotAgent(llm,
		agentTools,
		agents.WithMaxIterations(3),
		agents.WithPromptFormatInstructions(promptFormat),
		agents.WithPromptPrefix(promptPrefix),
		agents.WithPromptSuffix(promptSuffix),
		// agents.NewOpenAIOption().WithSystemMessage(systemPrompt)
	)
	executor := agents.NewExecutor(agent)

	fmt.Println("Welcome to the Halyard, the OLM v1 chatbot! Type 'exit' to quit.")
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

		answer, err := chains.Run(context.Background(), executor, userInput)
		if err != nil {
			answer = fmt.Sprintf("⚠️ there was an error executing your command, please try again: %v", err)
		}
		fmt.Printf("Halyard: %s\n", strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(answer, `\033`, "\033"), `\n`, "\n")))
	}
}
