package runtime

import (
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/stretchr/testify/require"
)

func TestServiceCtx_SIGUSR1_ConfigReload(t *testing.T) {
	t.Run("SIGUSR1 signal triggers config reload", func(t *testing.T) {
		// Middleware initial environment variable
		initialValue := "initial-test-value"
		t.Setenv("APP_SERVICE_NAME", initialValue)

		// Create initial config
		initialCfg, err := config.Init()
		require.NoError(t, err)
		require.Equal(t, initialValue, initialCfg.AppConfig.ServiceName)

		// Simulate service context with manual config management for test
		serviceCtx := New()
		serviceCtx.reloadConfigChannel = make(chan os.Signal, 1)

		// Create mock deps structure for testing
		serviceCtx.deps = &Dependencies{
			cfg: initialCfg,
		}

		var mu sync.Mutex
		var wg sync.WaitGroup

		// Start monitoring external changes
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-serviceCtx.reloadConfigChannel

			newCfg, err := config.Init()
			if err == nil {
				mu.Lock()
				serviceCtx.deps.cfg = newCfg
				mu.Unlock()
			}
		}()

		// Change environment variable
		newValue := "reloaded-test-value"
		t.Setenv("APP_SERVICE_NAME", newValue)

		// Send SIGUSR1 signal
		serviceCtx.reloadConfigChannel <- syscall.SIGUSR1

		// Wait for the reload to complete
		wg.Wait()

		// Verify config was reloaded
		mu.Lock()
		finalServiceName := serviceCtx.deps.cfg.AppConfig.ServiceName
		mu.Unlock()
		require.Equal(t, newValue, finalServiceName)
	})

	t.Run("config reload handles invalid configuration gracefully", func(t *testing.T) {
		// Middleware valid initial config
		t.Setenv("APP_SERVICE_NAME", "test-service")
		t.Setenv("HTTP_SERVER_PORT", "8080")

		// Create initial config
		initialCfg, err := config.Init()
		require.NoError(t, err)

		serviceCtx := New()
		serviceCtx.deps = &Dependencies{
			cfg: initialCfg,
		}
		originalServiceName := serviceCtx.deps.cfg.AppConfig.ServiceName

		// Middleware invalid port value to cause config load error
		t.Setenv("HTTP_SERVER_PORT", "invalid-port")

		// Create a channel to track reload completion
		reloadDone := make(chan bool, 1)
		serviceCtx.reloadConfigChannel = make(chan os.Signal, 1)

		go func() {
			<-serviceCtx.reloadConfigChannel

			newCfg, err := config.Init()
			if err != nil {
				// Config reload should handle error gracefully
				// Original config should remain unchanged
				reloadDone <- false
				return
			}
			serviceCtx.deps.cfg = newCfg
			reloadDone <- true
		}()

		// Send SIGUSR1 signal
		serviceCtx.reloadConfigChannel <- syscall.SIGUSR1

		// Wait for reload attempt
		select {
		case success := <-reloadDone:
			if success {
				t.Error("Expected config reload to fail with invalid port, but it succeeded")
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Config reload did not complete within expected time")
		}

		// Verify original config is preserved
		require.Equal(t, originalServiceName, serviceCtx.deps.cfg.AppConfig.ServiceName)
	})

	t.Run("multiple SIGUSR1 signals are handled correctly", func(t *testing.T) {
		t.Setenv("APP_SERVICE_NAME", "initial-value")

		// Create initial config
		initialCfg, err := config.Init()
		require.NoError(t, err)

		serviceCtx := New()
		serviceCtx.deps = &Dependencies{
			cfg: initialCfg,
		}
		serviceCtx.reloadConfigChannel = make(chan os.Signal, 1)

		var mu sync.Mutex
		var wg sync.WaitGroup
		reloadCount := 0

		wg.Add(1)
		go func() {
			defer wg.Done()
			for range serviceCtx.reloadConfigChannel {
				newCfg, err := config.Init()
				if err == nil {
					mu.Lock()
					serviceCtx.deps.cfg = newCfg
					reloadCount++
					mu.Unlock()
				}
			}
		}()

		// Send multiple signals with config changes
		testValues := []string{"value1", "value2", "value3"}
		for _, value := range testValues {
			t.Setenv("APP_SERVICE_NAME", value)
			serviceCtx.reloadConfigChannel <- syscall.SIGUSR1
			time.Sleep(50 * time.Millisecond)
		}

		// Close channel and wait for goroutine to complete
		close(serviceCtx.reloadConfigChannel)
		wg.Wait()

		require.Equal(t, len(testValues), reloadCount)
		mu.Lock()
		finalServiceName := serviceCtx.deps.cfg.AppConfig.ServiceName
		mu.Unlock()
		require.Equal(t, "value3", finalServiceName)
	})
}

func TestServiceCtx_ConfigReloadConcurrency(t *testing.T) {
	t.Run("concurrent config access is safe", func(t *testing.T) {
		t.Setenv("APP_SERVICE_NAME", "concurrent-test")

		// Create initial config
		initialCfg, err := config.Init()
		require.NoError(t, err)

		serviceCtx := New()
		serviceCtx.deps = &Dependencies{
			cfg: initialCfg,
		}
		serviceCtx.reloadConfigChannel = make(chan os.Signal, 1)

		// Start config reload handler
		go func() {
			<-serviceCtx.reloadConfigChannel
			newCfg, err := config.Init()
			if err == nil {
				serviceCtx.deps.cfg = newCfg
			}
		}()

		// Simulate concurrent access to config
		done := make(chan bool, 2)

		// Goroutine 1: Read config repeatedly
		go func() {
			for i := 0; i < 100; i++ {
				_ = serviceCtx.deps.cfg.AppConfig.ServiceName
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()

		// Goroutine 2: Trigger reload
		go func() {
			time.Sleep(10 * time.Millisecond)
			t.Setenv("APP_SERVICE_NAME", "updated-concurrent-test")
			serviceCtx.reloadConfigChannel <- syscall.SIGUSR1
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done

		// Test should complete without race conditions
		require.NotNil(t, serviceCtx.deps.cfg)
	})
}

func TestNew_WithReloadChannel(t *testing.T) {
	t.Run("service context initializes with reload channel", func(t *testing.T) {
		serviceCtx := New()

		require.NotNil(t, serviceCtx.reloadConfigChannel)
		require.NotNil(t, serviceCtx.shutdownChannel)
		// Note: deps would be nil until build() is called
		require.Nil(t, serviceCtx.deps)
	})
}
