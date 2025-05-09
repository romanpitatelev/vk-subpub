//nolint:errcheck
package subpub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultBuffer = 100
)

func TestSubPub(t *testing.T) {
	t.Run("create NewSubPub", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)
		if sp == nil {
			t.Fatal("NewSubPub() returned nil")
		}
	})
	t.Run("subscribe and publish", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		var received string

		sub, err := sp.Subscribe("test", func(msg interface{}) {
			received = msg.(string)
		})
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		defer sub.Unsubscribe()

		err = sp.Publish("test", "hello")
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		if received != "hello" {
			t.Errorf("expected 'hello', got %q", received)
		}
	})
	t.Run("unsubscribe successful", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		called := false
		sub, _ := sp.Subscribe("test", func(msg interface{}) {
			called = true
		})

		sub.Unsubscribe()
		sp.Publish("test", "should not be received")

		time.Sleep(100 * time.Millisecond)

		if called {
			t.Error("handler was called after unsubscribe")
		}
	})
	t.Run("close", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		_, err := sp.Subscribe("test", func(msg interface{}) {})
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		err = sp.Close(ctx)
		if err != nil {
			t.Fatalf("close failed: %v", err)
		}

		err = sp.Publish("test", "should fail")
		if !errors.Is(err, ErrSubPubClosed) {
			t.Errorf("expected ErrSubPubClosed  error after close, got %v", err)
		}
	})
	t.Run("close with unprocessed messages", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		var wg sync.WaitGroup

		wg.Add(1)

		_, err := sp.Subscribe("test", func(msg interface{}) {
			defer wg.Done()
			time.Sleep(500 * time.Millisecond)
		})
		if err != nil {
			t.Fatal(err)
		}

		sp.Publish("test", "message")

		ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			sp.Close(ctx)
			close(done)
		}()

		select {
		case <-done:
			wg.Wait()
		case <-time.After(400 * time.Millisecond):
			t.Fatal("close operation took too long")
		}
	})
	t.Run("several subscribers", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		var (
			count int
			mu    sync.Mutex
		)

		for range 5 {
			_, err := sp.Subscribe("test", func(msg interface{}) {
				mu.Lock()
				count++
				mu.Unlock()
			})
			if err != nil {
				t.Fatal(err)
			}
		}

		sp.Publish("test", "message")

		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		if count != 5 {
			t.Errorf("expected 5 handlers, got %d", count)
		}
		mu.Unlock()
	})
	t.Run("slow subscriber", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		slowDone := make(chan struct{})
		fastDone := make(chan struct{})

		_, err := sp.Subscribe("test", func(msg interface{}) {
			time.Sleep(200 * time.Millisecond)
			close(slowDone)
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err = sp.Subscribe("test", func(msg interface{}) {
			close(fastDone)
		})
		if err != nil {
			t.Fatal(err)
		}

		sp.Publish("test", "message")

		select {
		case <-fastDone:
		case <-time.After(100 * time.Millisecond):
			t.Error("Fast subscriber didn't receive message")
		}

		select {
		case <-slowDone:
		case <-time.After(300 * time.Millisecond):
			t.Error("slow subscriber didn't complete")
		}
	})
	t.Run("publish to nonexistent subject", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		err := sp.Publish("nonexistent", "message")
		if err != nil {
			t.Errorf("publish to nonexistent subject should not error, got %v", err)
		}
	})
	t.Run("subscribe after close", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)
		sp.Close(context.Background())

		_, err := sp.Subscribe("test", func(msg interface{}) {})
		if !errors.Is(err, ErrSubPubClosed) {
			t.Errorf("expected ErrSubPubClosed, got %v", err)
		}
	})
	t.Run("concurrent publish and subscribe", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		var wg sync.WaitGroup

		for range 10 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				sub, err := sp.Subscribe("test", func(msg interface{}) {})
				if err != nil {
					t.Error(err)

					return
				}

				defer sub.Unsubscribe()
			}()
		}

		for range 10 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				err := sp.Publish("test", "message")
				if err != nil {
					t.Error(err)
				}
			}()
		}

		wg.Wait()
	})
	t.Run("unsubscribe race", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		sub, err := sp.Subscribe("test", func(msg interface{}) {})
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()
			sub.Unsubscribe()
		}()

		go func() {
			defer wg.Done()
			sp.Publish("test", "message")
		}()

		wg.Wait()
	})
	t.Run("order of messages", func(t *testing.T) {
		sp := NewSubPub(defaultBuffer)

		var (
			results []int
			mu      sync.Mutex
			wg      sync.WaitGroup
		)

		wg.Add(10)

		sub, err := sp.Subscribe("test", func(msg interface{}) {
			mu.Lock()
			results = append(results, msg.(int))
			mu.Unlock()
			wg.Done()
		})
		if err != nil {
			t.Fatal(err)
		}
		defer sub.Unsubscribe()

		for i := range 10 {
			sp.Publish("test", i)
			time.Sleep(100 * time.Millisecond)
		}

		wg.Wait()
		mu.Lock()

		if len(results) != 10 {
			t.Errorf("expected 10 messages, got %d", len(results))
		}

		for i := range 10 {
			log.Debug().Msgf("results: %v", results)

			if results[i] != i {
				t.Errorf("out of order message at %d: got %d", i, results[i])

				break
			}
		}
		mu.Unlock()
	})
}
