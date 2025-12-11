package butterflymx

import (
	"context"
	"sync"
)

// APIStaticToken represents a static ButterflyMX API token.
type APIStaticToken string

var _ APITokenSource = APIStaticToken("")

// APIToken returns the token as a string.
func (t APIStaticToken) APIToken(ctx context.Context, _ bool) (APIStaticToken, error) {
	return t, nil
}

// APITokenSource is an interface for acquiring a ButterflyMX API token.
type APITokenSource interface {
	// APIToken should return a valid API token or an error.
	//
	// If [renew] is true, the implementation should attempt to renew the token
	// even if a cached token is available. Implementations may ignore this
	// parameter, and the caller must detect that the "new" token is still
	// invalid.
	APIToken(ctx context.Context, renew bool) (APIStaticToken, error)
}

// ReuseAPITokenSource returns a new [APITokenSource] that obeys the [renew]
// parameter. If [src] is already a reused token source, it is returned as-is.
func ReuseAPITokenSource(src APITokenSource) APITokenSource {
	if reused, ok := src.(*reusedAPITokenSource); ok {
		return reused
	}
	return &reusedAPITokenSource{
		new: src,
	}
}

type reusedAPITokenSource struct {
	mu  sync.RWMutex
	new APITokenSource
	old APIStaticToken
}

func (s *reusedAPITokenSource) APIToken(ctx context.Context, renew bool) (APIStaticToken, error) {
	if !renew {
		s.mu.RLock()
		token := s.old
		s.mu.RUnlock()

		if token != "" {
			return token, nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	s.old, err = s.new.APIToken(ctx, renew)
	if err != nil {
		return "", err
	}

	return s.old, nil
}
