package internal

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"weak"

	"go.yaml.in/yaml/v3"
)

type YAMLDocument struct {
	Root    *yaml.Node
	Aliases map[string]*yaml.Node
}

// YAMLCache represents a cache for a single YAML document and its aliases
type YAMLCache struct {
	documents sync.Map // map[string]weak.Pointer[YAMLDocument]
}

// NewYAMLCache creates a new YAML document cache
func NewYAMLCache() *YAMLCache {
	return &YAMLCache{}
}

// Store stores a YAML document in the cache, evaluating aliases
func (c *YAMLCache) Store(node *yaml.Node) (string, error) {
	newDoc := &YAMLDocument{
		Root:    node,
		Aliases: make(map[string]*yaml.Node),
	}
	c.extractAliases(node, newDoc.Aliases)
	content, err := yaml.Marshal(node)
	if err != nil {
		return "", err
	}
	key := hashContent(content)
	c.documents.Store(key, weak.Make(newDoc))
	return key, nil
}

// LoadDocument parses and caches a YAML document using the cleanup/weak pattern
func (c *YAMLCache) LoadDocument(content []byte) (*YAMLDocument, error) {
	var newDoc *YAMLDocument

	key := hashContent(content)
	for {
		// Try to load an existing document from the cache
		if value, ok := c.documents.Load(key); ok {
			// Check if the weak pointer still points to a valid document
			if doc := value.(weak.Pointer[YAMLDocument]).Value(); doc != nil {
				return doc, nil
			}
			// Weak pointer is nil, eagerly delete the stale entry
			c.documents.CompareAndDelete(key, value)
		}

		// No valid cached document found, create a new one if needed
		if newDoc == nil {
			var err error
			newDoc, err = c.parseDocument(content)
			if err != nil {
				return nil, fmt.Errorf("failed to parse YAML: %w", err)
			}
		}

		// Create a weak pointer to the new document
		wp := weak.Make(newDoc)

		// Try to install the new document in the cache
		if value, loaded := c.documents.LoadOrStore(key, wp); !loaded {
			// Successfully installed - add cleanup to remove cache entry when document is GC'd
			runtime.AddCleanup(newDoc, func(key string) {
				// Only delete if the weak pointer matches (handles race conditions)
				c.documents.CompareAndDelete(key, wp)
			}, key)
			return newDoc, nil
		} else {
			// Someone else installed a document first, check if it's still valid
			if doc := value.(weak.Pointer[YAMLDocument]).Value(); doc != nil {
				// Use the existing document, discard our newly created one
				return doc, nil
			}
			// The existing entry is stale, try again (will delete it in next iteration)
		}
	}
}

// parseDocument parses YAML content and extracts aliases
func (c *YAMLCache) parseDocument(content []byte) (*YAMLDocument, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	// Extract aliases from the document
	aliases := make(map[string]*yaml.Node)
	c.extractAliases(&root, aliases)

	return &YAMLDocument{
		Root:    &root,
		Aliases: aliases,
	}, nil
}

// extractAliases walks the YAML tree and extracts all aliases
func (c *YAMLCache) extractAliases(node *yaml.Node, aliases map[string]*yaml.Node) {
	anchors := make(map[string]*yaml.Node)
	c.walkNode(node, anchors, aliases)
}

// walkNode recursively walks the YAML node tree to find anchors and aliases
func (c *YAMLCache) walkNode(node *yaml.Node, anchors map[string]*yaml.Node, aliases map[string]*yaml.Node) {
	if node == nil {
		return
	}

	// If this node has an anchor, store it
	if node.Anchor != "" {
		anchors[node.Anchor] = node
		aliases[node.Anchor] = node
	}

	// If this is an alias node, resolve it
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		aliasName := node.Value
		if anchoredNode, exists := anchors[aliasName]; exists {
			aliases[aliasName] = anchoredNode
		}
	}

	// Recursively process child nodes
	for _, child := range node.Content {
		c.walkNode(child, anchors, aliases)
	}
}

// GetDocument retrieves a cached document (may return nil if GC'd)
func (c *YAMLCache) GetDocument(documentKey string) (*YAMLDocument, bool) {
	if value, ok := c.documents.Load(documentKey); ok {
		if doc := value.(weak.Pointer[YAMLDocument]).Value(); doc != nil {
			return doc, true
		}
		// Clean up stale entry
		c.documents.CompareAndDelete(documentKey, value)
	}
	return nil, false
}

// GetAlias retrieves a specific alias from a cached document
func (c *YAMLCache) GetAlias(documentKey string, aliasName string) (*yaml.Node, bool) {
	if doc, found := c.GetDocument(documentKey); found {
		if node, exists := doc.Aliases[aliasName]; exists {
			return node, true
		}
	}
	return nil, false
}

// GetAllAliases retrieves all aliases from a cached document
func (c *YAMLCache) GetAllAliases(documentKey string) (map[string]*yaml.Node, bool) {
	if doc, found := c.GetDocument(documentKey); found {
		return doc.Aliases, true
	}
	return nil, false
}

// Stats returns cache statistics
func (c *YAMLCache) Stats() (documents int, totalAliases int) {
	c.documents.Range(func(key, value interface{}) bool {
		if doc := value.(weak.Pointer[YAMLDocument]).Value(); doc != nil {
			documents++
			totalAliases += len(doc.Aliases)
		}
		return true
	})
	return documents, totalAliases
}

// CleanStaleEntries manually removes stale weak pointer entries
func (c *YAMLCache) CleanStaleEntries() int {
	var removed int
	c.documents.Range(func(key, value interface{}) bool {
		if value.(weak.Pointer[YAMLDocument]).Value() == nil {
			if c.documents.CompareAndDelete(key, value) {
				removed++
			}
		}
		return true
	})
	return removed
}

// hashContent creates a content hash for cache invalidation
func hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for brevity
}
