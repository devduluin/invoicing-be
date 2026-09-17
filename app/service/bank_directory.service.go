package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	meta "duluin_invoice/app/domain/meta"
	"duluin_invoice/utils"
)

// BankDirectoryService fetches the Duluin bank directory and caches it in
// process (reference data — small, changes rarely). A stale cache is served if
// the upstream is briefly unreachable.
type BankDirectoryService struct {
	url    string
	client *http.Client
	ttl    time.Duration

	mu     sync.RWMutex
	cached []meta.Bank
	expiry time.Time
}

func NewBankDirectoryService(url string) *BankDirectoryService {
	return &BankDirectoryService{
		url:    url,
		client: utils.NewOutboundHTTPClient(10 * time.Second),
		ttl:    24 * time.Hour,
	}
}

func (s *BankDirectoryService) ListBanks(ctx context.Context) ([]meta.Bank, error) {
	s.mu.RLock()
	fresh := time.Now().Before(s.expiry)
	cached := s.cached
	s.mu.RUnlock()
	if fresh && len(cached) > 0 {
		return cached, nil
	}

	banks, err := s.fetch(ctx)
	if err != nil {
		if len(cached) > 0 {
			return cached, nil // serve stale rather than fail
		}
		return nil, err
	}

	s.mu.Lock()
	s.cached = banks
	s.expiry = time.Now().Add(s.ttl)
	s.mu.Unlock()
	return banks, nil
}

func (s *BankDirectoryService) fetch(ctx context.Context) ([]meta.Bank, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("bank directory request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bank directory fetch: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bank directory: upstream status %d", res.StatusCode)
	}

	var body struct {
		Data []struct {
			Name      string `json:"name"`
			Code      string `json:"code"`
			SwiftCode string `json:"swiftcode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("bank directory decode: %w", err)
	}

	banks := make([]meta.Bank, 0, len(body.Data))
	for _, b := range body.Data {
		name := strings.TrimSpace(b.Name)
		if name == "" {
			continue
		}
		banks = append(banks, meta.Bank{
			Name:      name,
			Code:      strings.TrimSpace(b.Code),
			SwiftCode: strings.TrimSpace(b.SwiftCode),
		})
	}
	sort.Slice(banks, func(i, j int) bool { return banks[i].Name < banks[j].Name })
	return banks, nil
}
