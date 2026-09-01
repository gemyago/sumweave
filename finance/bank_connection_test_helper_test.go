package finance

import (
	"context"
	"errors"
	"sync"
)

type stubBankProvider struct {
	name         string
	startResult  ProviderLinkStart
	finishResult ProviderLinkResult
	finishParams []ProviderFinishLinkParams
	linkResult   ProviderTokenLinkResult
	startErr     error
	finishErr    error
	linkErr      error
	startCalls   int
	finishCalls  int
	linkedTokens []string
	mu           sync.Mutex
}

func (p *stubBankProvider) Name() string { return p.name }

func (p *stubBankProvider) StartLink(
	context.Context,
	ProviderStartLinkParams,
) (ProviderLinkStart, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startCalls++
	if p.startErr != nil {
		return ProviderLinkStart{}, p.startErr
	}
	if p.startResult.AuthorizationURL == "" && p.startResult.State == "" {
		return ProviderLinkStart{}, errors.New("start link unsupported")
	}
	return p.startResult, nil
}

func (p *stubBankProvider) FinishLink(
	_ context.Context,
	params ProviderFinishLinkParams,
) (ProviderLinkResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finishCalls++
	p.finishParams = append(p.finishParams, params)
	if p.finishErr != nil {
		return ProviderLinkResult{}, p.finishErr
	}
	if p.finishResult.Secret == "" && p.finishResult.ProviderReference == "" {
		return ProviderLinkResult{}, errors.New("finish link unsupported")
	}
	return p.finishResult, nil
}

func (p *stubBankProvider) LinkToken(
	_ context.Context,
	params ProviderTokenLinkParams,
) (ProviderTokenLinkResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.linkErr != nil {
		return ProviderTokenLinkResult{}, p.linkErr
	}
	p.linkedTokens = append(p.linkedTokens, params.Token)
	return p.linkResult, nil
}

var _ BankConnectionProvider = (*stubBankProvider)(nil)
