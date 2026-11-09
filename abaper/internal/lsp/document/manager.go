package document

import "sync"

// Document represents an open text document in the editor
type Document struct {
	URI        string
	LanguageID string
	Version    int
	Content    string
	ObjectType string
	ObjectName string
	IsSaved    bool
}

// Manager is a thread-safe store for open documents
type Manager struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

// NewManager creates a new document manager
func NewManager() *Manager {
	return &Manager{
		docs: make(map[string]*Document),
	}
}

// Open stores a newly opened document
func (m *Manager) Open(uri, languageID, content string, version int) *Document {
	objectType, objectName := ParseObjectFromURI(uri)
	doc := &Document{
		URI:        uri,
		LanguageID: languageID,
		Version:    version,
		Content:    content,
		ObjectType: objectType,
		ObjectName: objectName,
		IsSaved:    false,
	}
	m.mu.Lock()
	m.docs[uri] = doc
	m.mu.Unlock()
	return doc
}

// Update updates the content of an open document
func (m *Manager) Update(uri, content string, version int) *Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.docs[uri]
	if !ok {
		return nil
	}
	doc.Content = content
	doc.Version = version
	doc.IsSaved = false
	return doc
}

// Save marks a document as saved and optionally updates content
func (m *Manager) Save(uri string, content string) *Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.docs[uri]
	if !ok {
		return nil
	}
	if content != "" {
		doc.Content = content
	}
	doc.IsSaved = true
	return doc
}

// Close removes a document from the store
func (m *Manager) Close(uri string) {
	m.mu.Lock()
	delete(m.docs, uri)
	m.mu.Unlock()
}

// Get retrieves a document by URI
func (m *Manager) Get(uri string) (*Document, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, ok := m.docs[uri]
	return doc, ok
}
