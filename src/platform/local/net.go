package local

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

type network struct{}

func (network) Dial(ctx context.Context, host string, port int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (n network) Get(ctx context.Context, url string, header map[string]string) (status int, body []byte, err error) {
	return n.do(ctx, http.MethodGet, url, header, nil)
}

func (network) do(ctx context.Context, method, url string, header map[string]string, payload []byte) (status int, body []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err = io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}
