package clustercatalog

import (
	"context"
	"encoding/json"
	"fmt"
	llmTools "github.com/tmc/langchaingo/tools"
	"halyard/internal/cache"
	"io"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var clusterCatalog = schema.GroupVersionResource{
	Group:    "olm.operatorframework.io",
	Version:  "v1alpha1",
	Resource: "clustercatalogs",
}

type listClusterCatalogTool struct {
	client *dynamic.DynamicClient
}

func (t *listClusterCatalogTool) Name() string {
	return "list_cluster_catalogs"
}

func (t *listClusterCatalogTool) Description() string {
	return "List ClusterCatalogs on the cluster"
}

func (t *listClusterCatalogTool) Call(ctx context.Context, _ string) (string, error) {
	resp, err := t.client.Resource(clusterCatalog).List(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Sprintf("error calling tool: %v", err), nil
	}

	jsonBytes, err := json.Marshal(resp.Items)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func NewListTool(dynClient *dynamic.DynamicClient) llmTools.Tool {
	return &listClusterCatalogTool{
		client: dynClient,
	}
}

type getClusterCatalogTool struct {
	client *dynamic.DynamicClient
}

func (t *getClusterCatalogTool) Name() string {
	return "get_cluster_catalog"
}

func (t *getClusterCatalogTool) Description() string {
	return "Get a specific ClusterCatalog. Input must be in the format: { \"name\": <clusterextension-name> }. For example: { \"name\": \"operatorhubio\" }."
}

func (t *getClusterCatalogTool) Call(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Fatal(err)
	}

	resp, err := t.client.Resource(clusterCatalog).Get(ctx, args.Name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Sprintf("[]"), nil
		}
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func NewGetTool(dynClient *dynamic.DynamicClient) llmTools.Tool {
	return &getClusterCatalogTool{
		client: dynClient,
	}
}

type CatalogCacheTool struct {
	config *rest.Config
	cache  *cache.PackageCache
}

func NewCacheTool(config *rest.Config, packageCache *cache.PackageCache) llmTools.Tool {
	return &CatalogCacheTool{
		config: config,
		cache:  packageCache,
	}
}

func (t *CatalogCacheTool) Name() string {
	return "catalog_cache"
}

func (t *CatalogCacheTool) Description() string {
	return "Cache the contents of a cluster catalog. Input must be in the format: { \"endpoint\": <clusterextension-service-endpoint> }. For example: { \"endpoint\": \"https://catalogd-service.olmv1-system.svc/catalogs/operatorhubio\" }."
}

func (t *CatalogCacheTool) Call(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Fatal(err)
	}

	// Parse the URL
	parsedURL, err := url.Parse(args.Endpoint)
	if err != nil {
		return fmt.Sprintf("invalid URL: %v", err), nil
	}

	// Split the Host by "." to isolate the service name and namespace
	hostParts := strings.Split(parsedURL.Hostname(), ".")
	if len(hostParts) < 2 {
		return fmt.Sprintf("invalid URL format"), nil
	}

	// Extract the service name and namespace
	serviceName := hostParts[0]
	serviceNamespace := hostParts[1]
	targetPath, _ := url.JoinPath(parsedURL.Path, "api", "v1", "all")

	split := strings.Split(parsedURL.Path, "/")
	catalogName := split[len(split)-1]

	// API server proxy endpoint
	hostUrl, _ := url.Parse(t.config.Host)
	proxyURL := &url.URL{
		Scheme: "https",
		Host:   hostUrl.Host,
		Path:   fmt.Sprintf("/api/v1/namespaces/%s/services/https:%s:443/proxy%s", serviceNamespace, serviceName, targetPath),
	}

	tlsConfig, err := rest.TLSConfigFor(t.config)
	if err != nil {
		return fmt.Sprintf("could not create tlsconfig: %v", err), nil
	}

	// Create an HTTP client using Kubernetes TLS configuration
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// Create the HTTP request with Bearer token authentication
	req, err := http.NewRequest("GET", proxyURL.String(), nil)
	if err != nil {
		panic(err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+t.config.BearerToken)
	req.Header.Set("Accept", "application/json")

	// Execute the request
	resp, err := httpClient.Do(req)
	if err != nil {
		panic(err.Error())
	}
	defer resp.Body.Close()

	targetFilePath := path.Join(".halyard", fmt.Sprintf("%s.json", catalogName))
	_ = os.MkdirAll(filepath.Dir(targetFilePath), 0755)
	outfile, err := os.Create(targetFilePath)
	defer outfile.Close()
	if err != nil && !os.IsExist(err) {
		panic(err.Error())
	}
	io.Copy(outfile, resp.Body)

	catalogFile, err := os.Open(targetFilePath)
	defer catalogFile.Close()
	if err != nil {
		return fmt.Sprintf("error opening file %s: %v", targetFilePath, err), nil
	}
	if err := t.cache.CacheCatalog(catalogFile); err != nil {
		return fmt.Sprintf("error caching catalog: %v", err), nil
	}
	return fmt.Sprintf("Successfully cached catalog '%s'", catalogName), nil
}

type SearchCatalogTool struct {
	cache *cache.PackageCache
}

func NewSearchCatalogTool(packageCache *cache.PackageCache) llmTools.Tool {
	return &SearchCatalogTool{
		cache: packageCache,
	}
}

func (t *SearchCatalogTool) Name() string {
	return "search_catalog"
}

func (t *SearchCatalogTool) Description() string {
	return "Searches a catalog for bundles whose package name matches a regular expression. Input must be in the format: {\"catalogName\": <catalog_name>, \"expr\": <reg_exp>}. For example: {\"catalogName\": \"operatorhubio\", \"expr\": \".*prometheus.*\"}."
}

func (t *SearchCatalogTool) Call(ctx context.Context, arguments string) (string, error) {
	var args struct {
		CatalogName string `json:"catalogName"`
		Expr        string `json:"expr"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Fatal(err)
	}

	pattern := regexp.MustCompile(args.Expr)
	out := t.cache.Search(pattern)
	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return "error marshaling json", nil
	}
	return string(jsonBytes), nil
}
