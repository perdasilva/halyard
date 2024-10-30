package cache

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"regexp"
	"strings"
)

type Package string
type Version string
type CatalogCache map[Package][]Version

type PackageCache struct {
	cache CatalogCache
}

func NewPackageCache() *PackageCache {
	return &PackageCache{
		cache: make(CatalogCache),
	}
}

func (pc *PackageCache) Search(expr *regexp.Regexp) map[Package][]Version {
	out := make(map[Package][]Version)
	for pkg, versions := range pc.cache {
		if expr.MatchString(string(pkg)) {
			out[pkg] = versions
		}
	}
	return out
}

func (pc *PackageCache) CacheCatalog(catalog io.Reader) error {
	reader := bufio.NewReader(catalog)
	for {
		// Read until a newline character (assuming each JSON document is on a new line)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// If EOF is reached, break out of the loop
			if err.Error() == "EOF" {
				break
			}
			log.Fatalf("Failed to read line: %v", err)
		}

		// Check if line contains a JSON object (skip empty lines)
		if bytes.TrimSpace(line) == nil {
			continue
		}

		// Parse each JSON document
		var jsonData map[string]interface{}
		if err := json.Unmarshal(line, &jsonData); err != nil {
			log.Printf("Failed to parse JSON document: %v", err)
			continue
		}

		if _, ok := jsonData["schema"]; !ok || jsonData["schema"] != "olm.bundle" {
			continue
		}

		nameValue, ok := jsonData["name"]
		if !ok {
			continue
		}
		name, ok := nameValue.(string)
		if !ok {
			continue
		}

		idx := strings.Index(name, ".")
		packageName := Package(name[:idx])
		version := Version(name[idx+1:])
		pc.cache[packageName] = append(pc.cache[packageName], version)
	}

	return nil
}
