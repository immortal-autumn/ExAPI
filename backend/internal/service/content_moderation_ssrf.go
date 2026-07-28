package service

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/outbound"
)

var contentModerationOutboundPolicy = outbound.PublicPolicy()

func newContentModerationHTTPClient(policy outbound.Policy) *http.Client {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		baseTransport = &http.Transport{}
	}
	transport := baseTransport.Clone()
	// The moderation request carries a bearer credential and user input. Never
	// delegate its actual destination to environment proxy variables.
	transport.Proxy = nil
	_ = policy.ConfigureTransport(transport, outbound.DirectResolution)
	return &http.Client{
		Transport:     transport,
		CheckRedirect: policy.RedirectChecker(false, 10),
	}
}
