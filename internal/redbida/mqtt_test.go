package redbida

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type fakeMQTTToken struct {
	done chan struct{}
	err  error
}

func completedToken(err error) *fakeMQTTToken {
	done := make(chan struct{})
	close(done)
	return &fakeMQTTToken{done: done, err: err}
}

func (t *fakeMQTTToken) Wait() bool { <-t.done; return true }
func (t *fakeMQTTToken) WaitTimeout(d time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(d):
		return false
	}
}
func (t *fakeMQTTToken) Done() <-chan struct{} { return t.done }
func (t *fakeMQTTToken) Error() error          { return t.err }

type fakeMQTTMessage struct {
	topic    string
	payload  []byte
	retained bool
}

func (m fakeMQTTMessage) Duplicate() bool   { return false }
func (m fakeMQTTMessage) Qos() byte         { return 0 }
func (m fakeMQTTMessage) Retained() bool    { return m.retained }
func (m fakeMQTTMessage) Topic() string     { return m.topic }
func (m fakeMQTTMessage) MessageID() uint16 { return 0 }
func (m fakeMQTTMessage) Payload() []byte   { return m.payload }
func (m fakeMQTTMessage) Ack()              {}

type fakeMQTTClient struct {
	mu        sync.Mutex
	connected bool
	handler   mqtt.MessageHandler
	onPublish func(topic string, payload []byte, handler mqtt.MessageHandler)
}

func (c *fakeMQTTClient) IsConnected() bool      { return c.connected }
func (c *fakeMQTTClient) IsConnectionOpen() bool { return c.connected }
func (c *fakeMQTTClient) Connect() mqtt.Token    { c.connected = true; return completedToken(nil) }
func (c *fakeMQTTClient) Disconnect(uint)        { c.connected = false }
func (c *fakeMQTTClient) Publish(topic string, _ byte, _ bool, payload interface{}) mqtt.Token {
	c.mu.Lock()
	handler := c.handler
	c.mu.Unlock()
	if c.onPublish != nil {
		c.onPublish(topic, payload.([]byte), handler)
	}
	return completedToken(nil)
}
func (c *fakeMQTTClient) Subscribe(_ string, _ byte, handler mqtt.MessageHandler) mqtt.Token {
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()
	return completedToken(nil)
}
func (c *fakeMQTTClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
	return completedToken(errors.New("not implemented"))
}
func (c *fakeMQTTClient) Unsubscribe(...string) mqtt.Token        { return completedToken(nil) }
func (c *fakeMQTTClient) AddRoute(string, mqtt.MessageHandler)    {}
func (c *fakeMQTTClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

func TestMQTTReadIgnoresRetainedAndUnrelatedAcknowledgements(t *testing.T) {
	client := &fakeMQTTClient{connected: true}
	client.onPublish = func(_ string, _ []byte, handler mqtt.MessageHandler) {
		handler(client, fakeMQTTMessage{topic: "/private/i_gets/ack", retained: true, payload: []byte(`{"info":{"logo_header":"stale"}}`)})
		handler(client, fakeMQTTMessage{topic: "/private/i_gets/ack", payload: []byte(`{"info":{"other":"wrong"}}`)})
		handler(client, fakeMQTTMessage{topic: "/private/i_gets/ack", payload: []byte(`{"info":{"logo_header":"https://example.test/logo.png"}}`)})
	}
	broker := NewMQTTBroker(MQTTOptions{Timeout: time.Second})
	broker.client = client
	values, err := broker.Read(context.Background(), []string{"logo_header"})
	if err != nil || values["logo_header"] != "https://example.test/logo.png" {
		t.Fatalf("read values=%+v err=%v", values, err)
	}
}

func TestMQTTWriteWaitsForMatchingAcknowledgement(t *testing.T) {
	client := &fakeMQTTClient{connected: true}
	client.onPublish = func(_ string, _ []byte, handler mqtt.MessageHandler) {
		handler(client, fakeMQTTMessage{topic: "/private/i_sets/ack", payload: []byte(`{"info":`)})
		handler(client, fakeMQTTMessage{topic: "/private/i_sets/ack", payload: []byte(`{"info":{"logo_header":{"oldValue":"old","newValue":"wrong"}}}`)})
		handler(client, fakeMQTTMessage{topic: "/private/i_sets/ack", payload: []byte(`{"info":{"logo_header":{"oldValue":"old","newValue":"https://example.test/new.png"}}}`)})
	}
	broker := NewMQTTBroker(MQTTOptions{Timeout: time.Second})
	broker.client = client
	acks, err := broker.Write(context.Background(), map[string]any{"logo_header": "https://example.test/new.png"})
	if err != nil || acks["logo_header"].NewValue != "https://example.test/new.png" {
		t.Fatalf("write ack=%+v err=%v", acks, err)
	}
}

func TestMQTTAcknowledgementTimeoutIsTyped(t *testing.T) {
	client := &fakeMQTTClient{connected: true}
	broker := NewMQTTBroker(MQTTOptions{Timeout: 20 * time.Millisecond})
	broker.client = client
	_, err := broker.Read(context.Background(), []string{"logo_header"})
	var timeoutErr *AckTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("timeout error type = %T, %v", err, err)
	}
}

func TestMQTTDefaultsAndDecodeErrors(t *testing.T) {
	broker := NewMQTTBroker(MQTTOptions{})
	if broker.opts.Host != "127.0.0.1" || broker.opts.Port != 12369 || broker.opts.ReadTopic != "/private/i_gets" || broker.opts.WriteTopic != "/private/i_sets" {
		t.Fatalf("unexpected defaults: %+v", broker.opts)
	}
	if _, err := rawMap(map[string]json.RawMessage{"bad": []byte(`{`)}); err == nil {
		t.Fatal("malformed raw value was accepted")
	}
	if got := decodeRaw(json.RawMessage(`not-json`)); got != "not-json" {
		t.Fatalf("decodeRaw fallback = %#v", got)
	}
}
