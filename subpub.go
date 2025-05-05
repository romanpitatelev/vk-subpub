package subpub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const (
	defaultBuffer = 100
)

var ErrSubPubClosed = errors.New("subpub is closed")

type SubPubImpl struct {
	subjects   map[string][]*subscription
	mu         sync.RWMutex
	wg         sync.WaitGroup
	closed     atomic.Bool
	nextID     uint64
	chanBuffer int
}

func NewSubPub() *SubPubImpl {
	sp := &SubPubImpl{
		subjects:   make(map[string][]*subscription),
		chanBuffer: defaultBuffer,
	}

	return sp
}

type subscription struct {
	id      uint64
	subject string
	ch      chan interface{}
	handler MessageHandler
	parent  *SubPubImpl
	once    sync.Once
}

type MessageHandler func(msg interface{})

type Subscription interface {
	Unsubscribe()
}

type SubPub interface {
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	Publish(subject string, msg interface{}) error
	Close(ctx context.Context) error
}

func (sp *SubPubImpl) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	if sp.closed.Load() {
		return nil, ErrSubPubClosed
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	id := atomic.AddUint64(&sp.nextID, 1)
	subscript := &subscription{
		id:      id,
		subject: subject,
		ch:      make(chan interface{}, sp.chanBuffer),
		handler: cb,
		parent:  sp,
	}

	sp.subjects[subject] = append(sp.subjects[subject], subscript)

	go func() {
		for msg := range subscript.ch {
			subscript.handler(msg)
		}
	}()

	return subscript, nil
}

func (sp *SubPubImpl) Publish(subject string, msg interface{}) error {
	if sp.closed.Load() {
		return ErrSubPubClosed
	}

	sp.mu.RLock()
	defer sp.mu.RUnlock()

	subscriptions, ok := sp.subjects[subject]
	if !ok {
		return nil
	}

	for _, sub := range subscriptions {
		sp.wg.Add(1)
		go func(s *subscription) {
			defer sp.wg.Done()
			defer func() {
				if r := recover(); r != nil {
				}
			}()
			select {
			case sub.ch <- msg:
			default:
			}
		}(sub)
	}

	return nil
}

func (sp *SubPubImpl) Close(ctx context.Context) error {
	if !sp.closed.CompareAndSwap(false, true) {
		return nil
	}

	sp.mu.Lock()

	var allSubscriptions []*subscription

	for _, subscriptions := range sp.subjects {
		allSubscriptions = append(allSubscriptions, subscriptions...)
	}
	sp.mu.Unlock()

	for _, subscription := range allSubscriptions {
		subscription.Unsubscribe()
	}

	done := make(chan struct{})
	go func() {
		sp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *subscription) Unsubscribe() {
	s.once.Do(func() {
		s.parent.unsubscribe(s.id)
	})
}

func (sp *SubPubImpl) unsubscribe(id uint64) {
	if sp.closed.Load() {
		return
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	for subject, subscriptions := range sp.subjects {
		for i, subscription := range subscriptions {
			if subscription.id == id {
				close(subscription.ch)
				sp.subjects[subject] = append(subscriptions[:i], subscriptions[i+1:]...)

				if len(sp.subjects[subject]) == 0 {
					delete(sp.subjects, subject)
				}
				return
			}
		}
	}
}
