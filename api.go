package easemob

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"uw/ureq"
)

type refreshTokenReq struct {
	GrantType    string `json:"grant_type"`    // 授权方式。该参数设置为固定字符串 client_credentials，即客户端凭证模式。
	ClientId     string `json:"client_id"`     // App 的 client_id，用于生成 app token 调用 REST API。详见 环信即时通讯云控制台的应用详情页面。
	ClientSecret string `json:"client_secret"` // App 的 client_secret，用于生成 app token 调用 REST API。详见 环信即时通讯云控制台的应用详情页面。
	TTL          int    `json:"ttl"`           // token 有效期，单位为秒。若传入该参数，token 有效期以传入的值为准。若不传该参数，以 环信即时通讯云控制台的用户认证页面的 token 有效期的设置为准。 若设置为 0，则 token 永久有效。
}

type refreshTokenResp struct {
	Application string `json:"application"`  // 当前 App 的 UUID 值。
	AccessToken string `json:"access_token"` // 有效的 Token 字符串。
	ExpiresIn   int    `json:"expires_in"`   // Token 有效时间，单位为秒，在有效期内不需要重复获取。
}

func (eb *Easemob) RefreshToken(ctx context.Context, ttl int) error {
	c, e := eb.GetBaseClient(ctx)
	if e != nil {
		return fmt.Errorf("get client error: %w", e)
	}

	res, e := c.Post(eb.GetURL("token").String()).
		Set(ureq.ContentType, "application/json").
		Set(ureq.Accept, "application/json").
		Send(&refreshTokenReq{
			GrantType:    "client_credentials",
			ClientId:     eb.clientId,
			ClientSecret: eb.clientSecret,
			TTL:          ttl,
		}).End()
	if e != nil {
		return fmt.Errorf("refresh token error: %w", e)
	}

	if !res.OK() {
		text, _ := res.Text()
		return fmt.Errorf("refresh token error: %s, %s", res.Status, text)
	}

	resp := &refreshTokenResp{}
	if e = res.JSON(resp); e != nil {
		return fmt.Errorf("refresh token error: %w", e)
	}

	if len(strings.TrimSpace(resp.AccessToken)) < 1 {
		return errors.New("refresh token error: access token is empty")
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.accessToken = resp.AccessToken
	eb.accessTokenExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	return nil
}

type PushReqCommon struct {
	Targets     []string     `json:"targets,omitempty"` // 推送的目标用户 ID。最多可传 100 个。
	Strategy    int          `json:"strategy"`          // 推送策略: 0-4 具体参考: https://doc.easemob.com/push/push_send_notification.html#http-%E8%AF%B7%E6%B1%82
	PushMessage *PushMessage `json:"pushMessage"`       // 推送通知。关于通知内容，请查看 https://doc.easemob.com/push/push_notification_config.html
}

type PushMessage struct {
	Title   string      `json:"title"`            // 通知栏展示的通知标题，默认为“您有一条新消息”。该字段长度不能超过 32 个字符（一个汉字相当于两个字符）。
	Content string      `json:"content"`          // 通知栏展示的通知内容。默认为“请及时查看”。该字段长度不能超过 100 个字符（一个汉字相当于两个字符）。
	Ext     interface{} `json:"ext,omitempty"`    // 推送自定义扩展信息，为自定义 key-value 键值对。键值对个数不能超过 10 且长度不能超过 1024 个字符。
	Config  *PushConfig `json:"config,omitempty"` // 与用户点击通知相关的操作。以及角标的配置，包含 clickAction 和 badge 字段。
}

type PushConfig struct {
	ClickAction *PushConfigClickAction `json:"clickAction,omitempty"` // 在通知栏中点击触发的动作
	Badge       *PushConfigBadge       `json:"badge,omitempty"`       // 推送角标
}

// 兼容性: Android, iOS
// 环信 iOS 推送通道只支持设置为 url。
type PushConfigClickAction struct {
	Url      string `json:"url,omitempty"`      // 打开自定义的 URL
	Action   string `json:"action,omitempty"`   // 打开应用的指定页面
	Activity string `json:"activity,omitempty"` // 打开应用包名或 Activity 组件路径。若不传该字段，默认打开应用的首页
}

// 兼容性: Android
type PushConfigBadge struct {
	AddNum   int64  `json:"addNum,omitempty"`   // 表示推送通知到达设备时，角标数字累加的值。
	SetNum   int64  `json:"setNum,omitempty"`   // 表示推送通知到达设备时，角标数字设置的值。
	Activity string `json:"activity,omitempty"` // 入口类（华为角标需要配置）。
}

type PushRespCommon[T any] struct {
	Timestamp int  `json:"timestamp"` // Unix 时间戳，单位为毫秒。
	Data      []*T `json:"data"`      // 通知推送响应数据。
	Duration  int  `json:"duration"`  // 从发送请求到响应的时长，单位为毫秒。
}

type PushSyncRespData struct {
	PushStatus string                  `json:"pushStatus"` // 推送状态: SUCCESS: 推送成功, FAIL: 推送失败, ERROR: 推送异常, 具体: https://doc.easemob.com/push/push_send_notification.html#http-%E5%93%8D%E5%BA%94
	Data       *PushSyncRespStatusData `json:"data"`       // 推送结果。服务器根据推送结果判断推送状态。
}

type PushSyncRespStatusData struct {
	Result string   `json:"result"` // 推送结果
	MsgId  []string `json:"msg_id"` // 消息 ID
}

// 以同步方式发送推送通知
// 调用该接口以同步方式推送消息时，环信或第三方推送厂商在推送消息后，会将推送结果发送给环信服务器。服务器根据收到的推送结果判断推送状态。 该接口调用频率默认为 1 次/秒
// strategy: 推送策略, target: 推送目标，msg: 推送消息
func (em *Easemob) PushSync(ctx context.Context, strategy int, target string, msg *PushMessage) (*PushRespCommon[PushSyncRespData], error) {
	c, e := em.GetAccessClient(ctx)
	if e != nil {
		return nil, fmt.Errorf("get client error: %w", e)
	}

	res, e := c.Post(em.GetURL(path.Join("push/sync", target)).String()).
		Set(ureq.ContentType, "application/json").
		Set(ureq.Accept, "application/json").
		Send(&PushReqCommon{
			Strategy:    strategy,
			PushMessage: msg,
		}).End()
	if e != nil {
		return nil, fmt.Errorf("push sync error: %w", e)
	}

	if !res.OK() {
		text, _ := res.Text()
		return nil, fmt.Errorf("push sync error: %s, %s", res.Status, text)
	}

	resp := &PushRespCommon[PushSyncRespData]{}
	if e = res.JSON(resp); e != nil {
		return nil, fmt.Errorf("push sync error: %w", e)
	}

	for _, v := range resp.Data {
		if v.PushStatus != "SUCCESS" {
			return nil, fmt.Errorf("push sync error: %s", v.PushStatus)
		}
	}

	return resp, nil
}

type PushSingleRespData struct {
	PushStatus string `json:"pushStatus"` // 推送状态：ASYNC_SUCCESS 表示推送成功。
	Data       string `json:"data"`       // 异步推送的结果，即成功或失败。
	Desc       string `json:"desc"`       // 推送结果的相关描述。
}

// 以异步方式批量发送推送通知
// 调用该接口以异步方式为指定的单个或多个用户进行消息推送。
// strategy: 推送策略, targets: 推送目标，msg: 推送消息
func (em *Easemob) PushSingle(ctx context.Context, strategy int, targets []string, msg *PushMessage) (*PushRespCommon[PushSingleRespData], error) {
	c, e := em.GetAccessClient(ctx)
	if e != nil {
		return nil, fmt.Errorf("get client error: %w", e)
	}

	if len(targets) > 100 {
		return nil, errors.New("push single error: targets length > 100")
	}

	res, e := c.Post(em.GetURL("push/single").String()).
		Set(ureq.ContentType, "application/json").
		Set(ureq.Accept, "application/json").
		Send(&PushReqCommon{
			Targets:     targets,
			Strategy:    strategy,
			PushMessage: msg,
		}).End()
	if e != nil {
		return nil, fmt.Errorf("push sync error: %w", e)
	}

	if !res.OK() {
		text, _ := res.Text()
		return nil, fmt.Errorf("push sync error: %s, %s", res.Status, text)
	}

	resp := &PushRespCommon[PushSingleRespData]{}
	if e = res.JSON(resp); e != nil {
		return nil, fmt.Errorf("push sync error: %w", e)
	}

	return resp, nil
}
