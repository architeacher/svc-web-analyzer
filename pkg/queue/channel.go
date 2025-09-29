package queue

import (
	"io"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// channel is used mainly to be able to generate mocks for the Channel behavior.
type channel interface {
	io.Closer

	exchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	queueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	queueBind(name, key, exchange string, noWait bool, args amqp.Table) error

	publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) <-chan amqp.Delivery

	cancel(consumer string, noWait bool) error
}

// amqpChannel is used mainly to be able to generate mocks for the AMQP behavior.
//
//nolint:interfacebloat // necessary for complete AMQP channel interface
type amqpChannel interface {
	io.Closer

	Cancel(consumer string, noWait bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	ExchangeDeclarePassive(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	ExchangeDelete(name string, ifUnused, noWait bool) error
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error)
	QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	Qos(prefetchCount, prefetchSize int, global bool) error
}

type channelQos struct {
	applied       bool
	prefetchCount int
	prefetchSize  int
	global        bool
}

type channelQueueBinding struct {
	queueName    string
	bindingKey   string
	exchangeName string
	noWait       bool
	args         amqp.Table
}

type channelQueue struct {
	name       string
	durable    bool
	autoDelete bool
	exclusive  bool
	noWait     bool
	args       amqp.Table
}

type channelExchange struct {
	name       string
	kind       string
	durable    bool
	autoDelete bool
	internal   bool
	noWait     bool
	args       amqp.Table
}

// ChannelWrapper is a wrapper around amqp091-go.Channel, providing a mechanism to reconnect.
type ChannelWrapper struct {
	amqpChan amqpChannel

	logger Logger

	mutex    *sync.Mutex
	canceled atomic.Bool
	closed   atomic.Bool

	chQos                    channelQos
	autoDeletedQueues        []channelQueue
	autoDeletedQueueBindings []channelQueueBinding
	autoDeletedExchanges     []channelExchange

	reconnectDelay time.Duration
}

// Close is a wrapper around amqp091-go.Channel.Close method, which closes a channel.
func (ch *ChannelWrapper) Close() error {
	defer ch.mutex.Unlock()
	ch.mutex.Lock()

	if ch.isClosed() {
		return amqp.ErrClosed
	}

	ch.closed.Store(true)

	return ch.amqpChan.Close()
}

func (ch *ChannelWrapper) cancel(consumer string, noWait bool) error {
	defer ch.mutex.Unlock()
	ch.mutex.Lock()

	err := ch.amqpChan.Cancel(consumer, noWait)
	if err != nil {
		return err
	}

	ch.canceled.Store(true)

	return nil
}

//nolint:revive // This method uses same number of arguments as amqp091 Channel.consume.
func (ch *ChannelWrapper) consume(
	queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table,
) <-chan amqp.Delivery {
	deliveries := make(chan amqp.Delivery)

	go func() {
		for {
			ch.mutex.Lock()
			d, err := ch.amqpChan.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
			ch.mutex.Unlock()
			if err != nil {
				if ch.logger != nil {
					ch.logger.Error().Err(err).Msg("failed to consume messages")
				}
				time.Sleep(ch.reconnectDelay)

				continue
			}

			for msg := range d {
				deliveries <- msg
			}

			// sleep before IsClose call. closed flag may not set before sleep.
			time.Sleep(ch.reconnectDelay)

			if ch.isClosed() || ch.isCanceled() {
				close(deliveries)

				return
			}
		}
	}()

	return deliveries
}

//nolint:revive // This method has the same arguments as Channel.ExchangeDeclare from amqp091-go lib.
func (ch *ChannelWrapper) exchangeDeclare(
	name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table,
) error {
	if autoDelete {
		ch.autoDeletedExchanges = append(ch.autoDeletedExchanges, channelExchange{
			name:       name,
			kind:       kind,
			durable:    durable,
			autoDelete: autoDelete,
			internal:   internal,
			noWait:     noWait,
			args:       args,
		})
	}

	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	return ch.amqpChan.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (ch *ChannelWrapper) publish(
	exchange, key string, mandatory, immediate bool, msg amqp.Publishing,
) error {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	return ch.amqpChan.Publish(exchange, key, mandatory, immediate, msg)
}

func (ch *ChannelWrapper) queueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	defer ch.mutex.Unlock()
	ch.mutex.Lock()

	bindingRestored := false

	for _, q := range ch.autoDeletedQueues {
		if name == q.name {
			ch.autoDeletedQueueBindings = append(ch.autoDeletedQueueBindings, channelQueueBinding{
				queueName:    name,
				exchangeName: exchange,
				bindingKey:   key,
				noWait:       noWait,
				args:         args,
			})

			bindingRestored = true

			break
		}
	}

	if !bindingRestored {
		for _, e := range ch.autoDeletedExchanges {
			if e.name == exchange {
				ch.autoDeletedQueueBindings = append(ch.autoDeletedQueueBindings, channelQueueBinding{
					queueName:    name,
					exchangeName: exchange,
					bindingKey:   key,
					noWait:       noWait,
					args:         args,
				})

				break
			}
		}
	}

	return ch.amqpChan.QueueBind(name, key, exchange, noWait, args)
}

// queueDeclare is a wrapper around amqp091-go.QueueDeclare it stores the auto-deleted queues, so they can be restored.
// See ChannelWrapper.queueBind for more details.
func (ch *ChannelWrapper) queueDeclare(
	name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table,
) (amqp.Queue, error) {
	if autoDelete {
		ch.autoDeletedQueues = append(ch.autoDeletedQueues, channelQueue{
			name:       name,
			durable:    durable,
			autoDelete: autoDelete,
			exclusive:  exclusive,
			noWait:     noWait,
			args:       args,
		})
	}

	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	return ch.amqpChan.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (ch *ChannelWrapper) isClosed() bool {
	return ch.closed.Load()
}

func (ch *ChannelWrapper) isCanceled() bool {
	return ch.canceled.Load()
}

// ExchangeDeclare is a public wrapper around exchangeDeclare
func (ch *ChannelWrapper) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	return ch.exchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

// QueueDeclare is a public wrapper around queueDeclare
func (ch *ChannelWrapper) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return ch.queueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

// QueueBind is a public wrapper around queueBind
func (ch *ChannelWrapper) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	return ch.queueBind(name, key, exchange, noWait, args)
}

// Publish is a public wrapper around publish
func (ch *ChannelWrapper) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	return ch.publish(exchange, key, mandatory, immediate, msg)
}

// Consume is a public wrapper around consume
func (ch *ChannelWrapper) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) <-chan amqp.Delivery {
	return ch.consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
}