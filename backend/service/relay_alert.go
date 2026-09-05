package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

// RelayAlert deliberately excludes request bodies, headers and raw upstream errors.
type RelayAlert struct {
	Model     string
	Group     string
	Code      string
	RequestID string
	Status    int
}

type relayAlertDelivery struct {
	alert         RelayAlert
	token, chatID string
}

type relayAlertNotifier struct {
	mu     sync.Mutex
	recent map[string]time.Time
	queue  chan relayAlertDelivery
	client *http.Client
}

var relayAlertsOnce sync.Once
var relayAlerts *relayAlertNotifier
var relayAlertTokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)
var relayAlertChatPattern = regexp.MustCompile(`^(-?[0-9]+|@[A-Za-z0-9_]+)$`)

// NotifyRelayFailure is best-effort and never waits for the Telegram network.
func NotifyRelayFailure(alert RelayAlert) {
	token, chatID := os.Getenv("RELAY_ALERT_TELEGRAM_BOT_TOKEN"), os.Getenv("RELAY_ALERT_TELEGRAM_CHAT_ID")
	if !relayAlertTokenPattern.MatchString(token) || !relayAlertChatPattern.MatchString(chatID) {
		return
	}
	relayAlertsOnce.Do(func() {
		client, err := newRelayAlertClient(os.Getenv("RELAY_ALERT_PROXY_URL"))
		if err != nil {
			common.SysError("Invalid Telegram relay alert proxy configuration; notifications disabled")
			return
		}
		relayAlerts = &relayAlertNotifier{
			recent: make(map[string]time.Time), queue: make(chan relayAlertDelivery, 64),
			client: client,
		}
		go func() {
			for delivery := range relayAlerts.queue {
				if err := relayAlerts.send(delivery); err != nil {
					// HTTP errors may contain the bot token in their URL; never log them.
					common.SysError("Telegram relay alert delivery failed; check bot permissions and network")
				}
			}
		}()
	})
	if relayAlerts == nil {
		return
	}
	relayAlerts.enqueue(relayAlertDelivery{alert: alert, token: token, chatID: chatID}, time.Now())
}

// A dedicated transport keeps alert egress separate from upstream model traffic.
func newRelayAlertClient(proxy string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		parsed, err := url.Parse(proxy)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" && parsed.Scheme != "socks5h") {
			return nil, errors.New("invalid relay alert proxy")
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

func (n *relayAlertNotifier) enqueue(delivery relayAlertDelivery, now time.Time) bool {
	a := &delivery.alert
	a.Model, a.Group, a.Code, a.RequestID = relayAlertField(a.Model), relayAlertField(a.Group), relayAlertField(a.Code), relayAlertField(a.RequestID)
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%s", a.Group, a.Model, a.Status, a.Code)
	n.mu.Lock()
	defer n.mu.Unlock()
	if until, ok := n.recent[key]; ok && now.Before(until) {
		return false
	}
	for k, until := range n.recent {
		if !now.Before(until) {
			delete(n.recent, k)
		}
	}
	if len(n.recent) >= 1024 {
		return false
	}
	select {
	case n.queue <- delivery:
		n.recent[key] = now.Add(5 * time.Minute)
		return true
	default:
		return false
	}
}

func relayAlertField(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	runes := []rune(value)
	if len(runes) > 128 {
		runes = runes[:128]
	}
	return string(runes)
}

func (n *relayAlertNotifier) send(delivery relayAlertDelivery) error {
	a := delivery.alert
	body, err := common.Marshal(map[string]interface{}{
		"chat_id":              delivery.chatID,
		"text":                 fmt.Sprintf("API 请求异常\n时间：%s\n模型：%s\n分组：%s\n状态码：%d\n错误码：%s\nRequest ID：%s\n同类告警在本节点 5 分钟内仅发送一次。", time.Now().UTC().Format(time.RFC3339), a.Model, a.Group, a.Status, a.Code, a.RequestID),
		"link_preview_options": map[string]bool{"is_disabled": true},
	})
	if err != nil {
		return errors.New("alert encoding failed")
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+delivery.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errors.New("alert request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := n.client.Do(req)
	if err != nil {
		return errors.New("alert transport failed")
	}
	defer res.Body.Close()
	var result struct {
		OK bool `json:"ok"`
	}
	if res.StatusCode != http.StatusOK || common.DecodeJson(io.LimitReader(res.Body, 65536), &result) != nil || !result.OK {
		return errors.New("alert rejected")
	}
	return nil
}
