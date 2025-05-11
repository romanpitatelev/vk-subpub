package subpub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

var ErrSubPubClosed = errors.New("subpub is closed")

type SubPubImpl struct {
	subjects   map[string]map[uint64]*subscription
	mu         sync.RWMutex
	wg         sync.WaitGroup
	closed     bool
	nextID     uint64
	chanBuffer int
}

func NewSubPub(defaultBuffer int) *SubPubImpl {
	sp := &SubPubImpl{
		subjects:   make(map[string]map[uint64]*subscription),
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
	if sp.closed {
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

	log.Info().Msgf("user %d has subscribed for the subject %s", subscript.id, subscript.subject)

	_, exists := sp.subjects[subject]
	if exists {
		sp.subjects[subject][id] = subscript
	} else {
		subjectMap := make(map[uint64]*subscription)
		subjectMap[id] = subscript
		sp.subjects[subject] = subjectMap
	}

	go func() {
		for msg := range subscript.ch {
			subscript.handler(msg)
		}
	}()

	return subscript, nil
}

func (sp *SubPubImpl) Publish(subject string, msg interface{}) error {
	if sp.closed {
		return ErrSubPubClosed
	}

	sp.mu.RLock()
	defer sp.mu.RUnlock()

	idMap, subjectExists := sp.subjects[subject]
	if !subjectExists {
		log.Debug().Str("subject", subject).Msg("no subscribers for the subject")

		return nil
	}

	for _, sub := range idMap {
		sp.wg.Add(1)

		go func(sub *subscription) {
			defer sp.wg.Done()

			select {
			case sub.ch <- msg:
			default:
			}
		}(sub)
	}

	return nil
}

func (sp *SubPubImpl) Close(ctx context.Context) error {
	sp.closed = true

	sp.mu.Lock()

	done := make(chan struct{})
	go func() {
		sp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("subpub shutdown has been interrupted: %w", ctx.Err())
	}
}

func (s *subscription) Unsubscribe() {
	s.once.Do(func() {
		s.parent.unsubscribe(s.id, s.subject)
	})
}

func (sp *SubPubImpl) unsubscribe(id uint64, subject string) {
	if sp.closed {
		return
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	idMap, subjectExists := sp.subjects[subject]
	if !subjectExists {
		log.Info().Msgf("subject %s is not found", subject)

		return
	}

	subscription, idExists := idMap[id]
	if !idExists {
		log.Info().Msgf("subcription id %d is not found in the subject %s", id, subject)

		return
	}

	close(subscription.ch)
	delete(idMap, id)
	log.Info().Msgf("removed subscription id %d from the subject '%s'", id, subject)

	if len(idMap) == 0 {
		log.Info().Msgf("removed empty subject '%s'", subject)
	}
}
