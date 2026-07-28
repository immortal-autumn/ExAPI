package service

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/outbound"
)

var contentModerationOutboundPolicy = outbound.PublicPolicy()

func newContentModerationHTTPClient(policy outbound.Policy) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The moderation request carries a bearer credential and user input. Never
	// delegate its actual destination to environment proxy variables.
	transport.Proxy = nil
	_ = policy.ConfigureTransport(transport, outbound.DirectResolution)
	return &http.Client{
		Transport:     transport,
		CheckRedirect: policy.RedirectChecker(false, 10),
	}
}
