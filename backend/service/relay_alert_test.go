package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type relayAlertTransport func(*http.Request) (*http.Response, error)

func (f relayAlertTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRelayAlertDeduplicatesWithoutRequestIDAndExpires(t *testing.T) {
	n := &relayAlertNotifier{recent: make(map[string]time.Time), queue: make(chan relayAlertDelivery, 4)}
	now := time.Unix(100, 0)
	d := relayAlertDelivery{alert: RelayAlert{Model: "gpt-5.6-sol", Group: "special", Code: "model_not_found", Status: 503, RequestID: "first"}}
	require.True(t, n.enqueue(d, now))
	d.alert.RequestID = "second"
	assert.False(t, n.enqueue(d, now.Add(time.Minute)))
	require.True(t, n.enqueue(d, now.Add(5*time.Minute)))
	d.alert.Model = "another-model"
	require.True(t, n.enqueue(d, now.Add(5*time.Minute)))
	assert.Len(t, n.queue, 3)
}

func TestRelayAlertFullQueueDoesNotConsumeDedupeWindow(t *testing.T) {
	n := &relayAlertNotifier{recent: make(map[string]time.Time), queue: make(chan relayAlertDelivery, 1)}
	n.queue <- relayAlertDelivery{}
	d := relayAlertDelivery{alert: RelayAlert{Code: "model_not_found"}}
	assert.False(t, n.enqueue(d, time.Unix(100, 0)))
	<-n.queue
	assert.True(t, n.enqueue(d, time.Unix(100, 0)))
}

func TestRelayAlertTelegramPayloadAndFailureResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"success", `{"ok":true}`, 200, true},
		{"bot rejected", `{"ok":false}`, 200, false},
		{"rate limited", `{"ok":false}`, 429, false},
		{"invalid response", `not json`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &relayAlertNotifier{client: &http.Client{Transport: relayAlertTransport(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "https://api.telegram.org/bot123:secret/sendMessage", req.URL.String())
				assert.Equal(t, http.MethodPost, req.Method)
				var payload struct {
					ChatID string `json:"chat_id"`
					Text   string `json:"text"`
				}
				require.NoError(t, common.DecodeJson(req.Body, &payload))
				assert.Equal(t, "-100123", payload.ChatID)
				assert.Contains(t, payload.Text, "model_not_found")
				assert.Contains(t, payload.Text, "request-123")
				assert.NotContains(t, payload.Text, "secret")
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}}
			err := n.send(relayAlertDelivery{token: "123:secret", chatID: "-100123", alert: RelayAlert{Model: "gpt-5.6-sol", Group: "special", Code: "model_not_found", RequestID: "request-123", Status: 503}})
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "secret")
			}
		})
	}
}
