package network

import (
	"sync"
	"time"
)

const (
	K           = 20  // bucket size
	ALPHA       = 3   // concurrency parameter
	ID_LENGTH   = 160 // SHA-1 hash length in bits
	BUCKET_SIZE = 20
)

// Bucket represents a k-bucket in the routing table
type Bucket struct {
	contacts []Contact
	mutex    sync.RWMutex
}

// NewBucket creates a new k-bucket
func NewBucket() *Bucket {
	return &Bucket{
		contacts: make([]Contact, 0, K),
	}
}

// AddContact adds a contact to the bucket
func (b *Bucket) AddContact(contact Contact) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Check if contact already exists
	for i, c := range b.contacts {
		if c.Node.ID == contact.Node.ID {
			// Update last seen time and move to end
			b.contacts[i].LastSeen = time.Now()
			// Move contact to end (most recently seen)
			updatedContact := b.contacts[i]
			b.contacts = append(b.contacts[:i], b.contacts[i+1:]...)
			b.contacts = append(b.contacts, updatedContact)
			return
		}
	}

	// Add new contact
	contact.LastSeen = time.Now()
	if len(b.contacts) < K {
		b.contacts = append(b.contacts, contact)
	} else {
		// Bucket is full, replace least recently seen (first element)
		if len(b.contacts) > 0 {
			b.contacts[0] = contact
		}
	}
}

// GetContacts returns all contacts in the bucket
func (b *Bucket) GetContacts() []Contact {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	contacts := make([]Contact, len(b.contacts))
	copy(contacts, b.contacts)
	return contacts
}
