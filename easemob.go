package easemob

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sync"
	"time"
	"uw/ureq"
)

type Easemob struct {
	mu     *sync.RWMutex // 全局锁
	exitCh chan struct{} // 退出通道
	client *ureq.Client  // HTTP 客户端

	baseURL *url.URL // 基础 URL
	orgName string   // 组织名称
	appName string   // 应用名称

	clientId     string // App 的 client_id
	clientSecret string // App 的 client_secret

	accessToken          string    // Token 字符串
	accessTokenExpiresAt time.Time // Token 有效时间

	limiterResetTicker *time.Ticker // 限流重置定时器
	limiterChan        chan bool    // 限流通道
}

// NewEasemob 创建 Easemob 实例
// host: 分配的 Easemob 服务器域名
// orgName: 组织名称
// appName: 应用名称
// clientId: App 的 client_id
// clientSecret: App 的 client_secret
func NewEasemob(host, orgName, appName, clientId, clientSecret string) (*Easemob, error) {
	if len(host) < 1 || len(orgName) < 1 || len(appName) < 1 ||
		len(clientId) < 1 || len(clientSecret) < 1 {
		return nil, errors.New("invalid params")
	}

	eb := &Easemob{
		mu:     &sync.RWMutex{},
		exitCh: make(chan struct{}),
		client: ureq.New(),

		baseURL: &url.URL{
			Scheme: "https",
			Host:   host,
		},
		orgName: orgName,
		appName: appName,

		clientId:     clientId,
		clientSecret: clientSecret,

		limiterResetTicker: time.NewTicker(time.Second),
		limiterChan:        make(chan bool, 1),
	}

	go eb.limiter()

	return eb, nil
}

func (eb *Easemob) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.limiterResetTicker.Stop()
	close(eb.limiterChan)
	eb.exitCh <- struct{}{}
}

// SetLimiter 设置限流
// rate: 限流速率
// interval: 限流间隔 (重置时间)
func (eb *Easemob) SetLimiter(rate uint32, interval time.Duration) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.limiterResetTicker = time.NewTicker(interval)
	eb.limiterChan = make(chan bool, rate)
}

func (eb *Easemob) limiter() {
	defer func() { _ = recover() }()

	for {
		select {
		case <-eb.limiterResetTicker.C:
			for i := 0; i < cap(eb.limiterChan); i++ {
				<-eb.limiterChan
			}
		case <-eb.exitCh:
			return
		}
	}
}

// SetClientTimeout 设置 HTTP 客户端超时时间
func (eb *Easemob) SetClientTimeout(timeout time.Duration) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.client = eb.client.Timeout(timeout)
}

func (eb *Easemob) GetBaseClient(ctx context.Context) (*ureq.Client, error) {
	if e := eb.getLimiter(ctx); e != nil {
		return nil, e
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return eb.client.Clone(), nil
}

func (eb *Easemob) GetAccessClient(ctx context.Context) (*ureq.Client, error) {
	if e := eb.getLimiter(ctx); e != nil {
		return nil, e
	}

	eb.mu.RLock()

	if eb.accessTokenExpiresAt.Before(time.Now()) || len(eb.accessToken) < 1 {
		eb.mu.RUnlock()
		if e := eb.RefreshToken(ctx, 0); e != nil {
			return nil, fmt.Errorf("refresh token error: %w", e)
		}

		eb.mu.RLock()
	}

	defer eb.mu.RUnlock()

	return eb.client.Clone().
		Set("Authorization", "Bearer "+eb.accessToken), nil
}

func (eb *Easemob) GetURL(subPath string) *url.URL {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.baseURL.ResolveReference(&url.URL{
		Path: path.Join(eb.orgName, eb.appName, subPath),
	})
}

func (eb *Easemob) getLimiter(ctx context.Context) error {
	select {
	case eb.limiterChan <- true:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
