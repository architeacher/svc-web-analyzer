package runtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("creates service context with default values", func(t *testing.T) {
		t.Parallel()

		serviceCtx := New()

		require.NotNil(t, serviceCtx)
		require.NotNil(t, serviceCtx.shutdownChannel)
		require.Nil(t, serviceCtx.deps)
		require.Nil(t, serviceCtx.serverReady)
	})

	t.Run("creates service context with options", func(t *testing.T) {
		t.Parallel()

		ch := make(chan os.Signal, 1)
		serviceCtx := New(
			WithServiceTermination(ch),
			WithWaitingForServer(),
		)

		require.NotNil(t, serviceCtx)
		require.Equal(t, ch, serviceCtx.shutdownChannel)
		require.NotNil(t, serviceCtx.serverReady)
	})
}

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	t.Run("creates publisher context with default values", func(t *testing.T) {
		t.Parallel()

		publisherCtx := NewPublisher()

		require.NotNil(t, publisherCtx)
		require.NotNil(t, publisherCtx.shutdownChannel)
		require.Nil(t, publisherCtx.deps)
	})

	t.Run("creates publisher context with options", func(t *testing.T) {
		t.Parallel()

		ch := make(chan os.Signal, 1)
		publisherCtx := NewPublisher(WithPublisherTermination(ch))

		require.NotNil(t, publisherCtx)
		require.Equal(t, ch, publisherCtx.shutdownChannel)
	})
}

func TestNewSubscriber(t *testing.T) {
	t.Parallel()

	t.Run("creates subscriber context with default values", func(t *testing.T) {
		t.Parallel()

		subscriberCtx := NewSubscriber()

		require.NotNil(t, subscriberCtx)
		require.NotNil(t, subscriberCtx.shutdownChannel)
		require.Nil(t, subscriberCtx.deps)
	})

	t.Run("creates subscriber context with options", func(t *testing.T) {
		t.Parallel()

		ch := make(chan os.Signal, 1)
		subscriberCtx := NewSubscriber(WithSubscriberTermination(ch))

		require.NotNil(t, subscriberCtx)
		require.Equal(t, ch, subscriberCtx.shutdownChannel)
	})
}
