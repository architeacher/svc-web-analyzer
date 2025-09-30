package runtime

import (
	"os"
)

type (
	ServiceOption func(*ServiceCtx)

	PublisherOption func(*PublisherCtx)

	SubscriberOption func(*SubscriberCtx)
)

func WithServiceTermination(ch chan os.Signal) ServiceOption {
	return func(ctx *ServiceCtx) {
		ctx.shutdownChannel = ch
	}
}

func WithPublisherTermination(ch chan os.Signal) PublisherOption {
	return func(ctx *PublisherCtx) {
		ctx.shutdownChannel = ch
	}
}

func WithSubscriberTermination(ch chan os.Signal) SubscriberOption {
	return func(ctx *SubscriberCtx) {
		ctx.shutdownChannel = ch
	}
}

func WithWaitingForServer() ServiceOption {
	return func(ctx *ServiceCtx) {
		ctx.serverReady = make(chan struct{})
	}
}
