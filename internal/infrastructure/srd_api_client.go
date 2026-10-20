package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"mpp-zmyslek/internal/domain"
)

// DefaultSRDAPIURL serves the 2014 (SRD 5.1) rules used by the assignment.
const DefaultSRDAPIURL = "https://www.dnd5eapi.co/api/2014"

// SRDAPIClient looks up spells and equipment in the external D&D 5e API.
//
// The API is run by volunteers, so the client sends at most one request per
// 200 ms (5 per second). The mutex protects the shared "last request" time:
// concurrent lookups queue up and are started one interval apart.
type SRDAPIClient struct {
	baseURL  string
	interval time.Duration
	client   *http.Client

	mu          sync.Mutex
	lastRequest time.Time
}

// NewSRDAPIClient creates a client for the API at baseURL.
func NewSRDAPIClient(baseURL string) *SRDAPIClient {
	return &SRDAPIClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		interval: 200 * time.Millisecond,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Lookup returns the API's JSON document for a resource ("spells" or
// "equipment") and a name such as "Fire Bolt". It returns an error wrapping
// domain.ErrNotFound when the API does not know the name, and ctx.Err() when
// the caller gives up while waiting.
func (c *SRDAPIClient) Lookup(ctx context.Context, resource, name string) (map[string]any, error) {
	if err := c.waitForTurn(ctx); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+resource+"/"+url.PathEscape(Slug(name)), nil)
	if err != nil {
		return nil, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("external API request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s %q %w in the external API", resource, name, domain.ErrNotFound)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external API returned %s", response.Status)
	}
	var document map[string]any
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("external API returned invalid JSON: %w", err)
	}
	return document, nil
}

// Slug turns a spell or item name into the API's index: "Crossbow, light"
// becomes "crossbow-light" and "Hunter's Mark" becomes "hunters-mark". The
// rules call half plate armor "half plate"; the API calls it "half-plate-armor".
func Slug(name string) string {
	var slug strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r == '\'':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if dash && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			slug.WriteRune(r)
			dash = false
		default:
			dash = true
		}
	}
	if slug.String() == "half-plate" {
		return "half-plate-armor"
	}
	return slug.String()
}

// waitForTurn holds the mutex while it waits until a full interval has passed
// since the previous request, so requests are spaced out even when many
// goroutines call Lookup at once.
func (c *SRDAPIClient) waitForTurn(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.interval - time.Since(c.lastRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.lastRequest = time.Now()
	return nil
}
