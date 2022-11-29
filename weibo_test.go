package weibo

import (
	"os"
	"testing"
)

func TestCrawl(t *testing.T) {
	c := Client{
		Cookie: os.Getenv("WEIBO_CLIENT_TEST_COOKIE"),
		Proxy:  os.Getenv("WEIBO_CLIENT_TEST_PROXY"),
	}
	if mblogs, err := c.GetMblogs(Huxijing, 1, true); err != nil {
		t.Error(err)
	} else {
		for _, mblog := range mblogs {
			t.Log(mblog)
		}
	}
}
