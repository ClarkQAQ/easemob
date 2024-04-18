### Easemob Go SDK (简易版)

> 由于这个 SDK 主要用户是中文用户, 所以本 README 全程中文!

这是一个实现了限流器的环信推送 Golang SDK, 目前支持的功能有:

1. 获取 Access Token (并且会自动刷新 Access Token)
2. 定向推送
3. 批量推送

但是没实现各厂商专有结构, 如有需要可以自行修改, 但请注意 License。


#### 使用方法


```go
package main

import (
	"context"
	"uw/ulog"

	"easemob"
)

func main() {
	eb, e := easemob.NewEasemob("xxxx", "xxxx", "xxx", "xxx", "xxxx")
	if e != nil {
		ulog.Fatal("init easemob error: %s", e)
	}

	defer eb.Close()

	// 放松限制器
	eb.SetLimiter(10, 1)

	{
		resp, e := eb.PushSync(context.Background(), 3, "1", &easemob.PushMessage{
			Title:   "测试批量推送",
			Content: "喵喵喵",
		})
		if e != nil {
			ulog.Fatal("push error: %s", e)
		}

		ulog.Info("push response: %+v", resp)
	}

	{
		resp, e := eb.PushSingle(context.Background(), 3, []string{"1", "2"}, &easemob.PushMessage{
			Title:   "测试批量推送",
			Content: "喵喵喵",
		})
		if e != nil {
			ulog.Fatal("push error: %s", e)
		}

		ulog.Info("push response: %+v", resp)
	}
}

```