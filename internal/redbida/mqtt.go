package redbida

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTOptions struct {
	Host          string
	Port          int
	ReadTopic     string
	ReadAckTopic  string
	WriteTopic    string
	WriteAckTopic string
	Timeout       time.Duration
}

type MQTTBroker struct {
	client mqtt.Client
	opts   MQTTOptions
	mu     sync.Mutex
}

// AckTimeoutError means the request was published but its legacy ack topic did
// not answer in time. Callers must treat the write outcome as uncertain and
// prefer read-back over blindly retrying a potentially disruptive key.
type AckTimeoutError struct {
	Topic string
}

func (e *AckTimeoutError) Error() string {
	return fmt.Sprintf("mqtt acknowledgement timeout on %s", e.Topic)
}

func NewMQTTBroker(opts MQTTOptions) *MQTTBroker {
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.Port == 0 {
		opts.Port = 12369
	}
	if opts.ReadTopic == "" {
		opts.ReadTopic = "/private/i_gets"
	}
	if opts.ReadAckTopic == "" {
		opts.ReadAckTopic = "/private/i_gets/ack"
	}
	if opts.WriteTopic == "" {
		opts.WriteTopic = "/private/i_sets"
	}
	if opts.WriteAckTopic == "" {
		opts.WriteAckTopic = "/private/i_sets/ack"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	return &MQTTBroker{opts: opts}
}

func (b *MQTTBroker) connect(ctx context.Context) error {
	if b.client != nil && b.client.IsConnected() {
		return nil
	}
	clientID := fmt.Sprintf("kspcam-redbida-%d", time.Now().UnixNano())
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", b.opts.Host, b.opts.Port)).
		SetClientID(clientID).
		SetCleanSession(true).
		SetAutoReconnect(false).
		SetConnectTimeout(b.opts.Timeout)
	b.client = mqtt.NewClient(opts)
	if err := waitToken(ctx, b.client.Connect(), "connect"); err != nil {
		return err
	}
	return nil
}

type ackDecoder func([]byte) (bool, error)

func (b *MQTTBroker) request(ctx context.Context, topic, ackTopic string, payload any, decode ackDecoder) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	opCtx, cancel := context.WithTimeout(ctx, b.opts.Timeout)
	defer cancel()
	if err := b.connect(opCtx); err != nil {
		return err
	}
	ackCh := make(chan []byte, 16)
	handler := func(_ mqtt.Client, msg mqtt.Message) {
		if msg.Retained() {
			return
		}
		select {
		case ackCh <- append([]byte(nil), msg.Payload()...):
		default:
		}
	}
	if err := waitToken(opCtx, b.client.Subscribe(ackTopic, 0, handler), "subscribe"); err != nil {
		return err
	}
	defer func() {
		if token := b.client.Unsubscribe(ackTopic); token != nil {
			token.WaitTimeout(500 * time.Millisecond)
		}
	}()
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mqtt marshal: %w", err)
	}
	if err := waitToken(opCtx, b.client.Publish(topic, 0, false, body), "publish"); err != nil {
		return err
	}
	for {
		select {
		case data := <-ackCh:
			matched, err := decode(data)
			if err != nil {
				// The legacy topic is shared and has no correlation ID. Treat a
				// malformed payload as unrelated and keep waiting; timeout handling
				// will perform a read-back for writes whose outcome is uncertain.
				continue
			}
			if matched {
				return nil
			}
		case <-opCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return &AckTimeoutError{Topic: ackTopic}
		}
	}
}

func waitToken(ctx context.Context, token mqtt.Token, action string) error {
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			return fmt.Errorf("mqtt %s: %w", action, err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *MQTTBroker) Read(ctx context.Context, keys []string) (map[string]any, error) {
	var out map[string]json.RawMessage
	err := b.request(ctx, b.opts.ReadTopic, b.opts.ReadAckTopic, map[string]any{"info": keys}, func(data []byte) (bool, error) {
		var ack struct {
			Info map[string]json.RawMessage `json:"info"`
		}
		if err := json.Unmarshal(data, &ack); err != nil {
			return false, err
		}
		if ack.Info == nil || !containsExactRaw(ack.Info, keys) {
			return false, nil
		}
		out = ack.Info
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return rawMap(out)
}

func (b *MQTTBroker) Write(ctx context.Context, changes map[string]any) (map[string]WriteAck, error) {
	var raw map[string]struct {
		OldValue json.RawMessage `json:"oldValue"`
		NewValue json.RawMessage `json:"newValue"`
	}
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	err := b.request(ctx, b.opts.WriteTopic, b.opts.WriteAckTopic, map[string]any{"info": changes}, func(data []byte) (bool, error) {
		var ack struct {
			Info map[string]struct {
				OldValue json.RawMessage `json:"oldValue"`
				NewValue json.RawMessage `json:"newValue"`
			} `json:"info"`
		}
		if err := json.Unmarshal(data, &ack); err != nil {
			return false, err
		}
		if !containsAllWriteAck(ack.Info, keys) {
			return false, nil
		}
		for key, expected := range changes {
			if !valuesEqual(decodeRaw(ack.Info[key].NewValue), expected) {
				return false, nil
			}
		}
		raw = ack.Info
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]WriteAck, len(raw))
	for key, item := range raw {
		out[key] = WriteAck{OldValue: decodeRaw(item.OldValue), NewValue: decodeRaw(item.NewValue)}
	}
	return out, nil
}

func containsExactRaw(values map[string]json.RawMessage, keys []string) bool {
	if len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			return false
		}
	}
	return true
}

func containsAllWriteAck(values map[string]struct {
	OldValue json.RawMessage `json:"oldValue"`
	NewValue json.RawMessage `json:"newValue"`
}, keys []string) bool {
	if values == nil {
		return false
	}
	if len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			return false
		}
	}
	return true
}

func rawMap(raw map[string]json.RawMessage) (map[string]any, error) {
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			return nil, fmt.Errorf("decode %s: %w", key, err)
		}
		out[key] = decoded
	}
	return out, nil
}

func decodeRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var out any
	if json.Unmarshal(raw, &out) != nil {
		return string(raw)
	}
	return out
}
