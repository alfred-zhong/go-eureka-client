package eureka

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/google/uuid"
)

// Client 代表一个简单的 Eureka Client
type Client struct {
	HostName string
	App      string

	IPAddr string
	Port   int
}

// NewClient 新建一个 Client
func NewClient(app string) (*Client, error) {
	if app == "" {
		return nil, errors.New("app 不能为空")
	}

	c := &Client{
		App: app,
	}
	c.autoFill()
	return c, nil
}

// NewClientWithPort 新建一个带有端口的 Client
func NewClientWithPort(app string, port int) (*Client, error) {
	if app == "" {
		return nil, errors.New("app 不能为空")
	}

	c := &Client{
		App:  app,
		Port: port,
	}
	c.autoFill()
	return c, nil
}

func (c *Client) autoFill() {
	// 尝试自动填充 IP
	if ip, ok := AutoIP4(); ok {
		c.IPAddr = ip
	}

	// 尝试自动填充 HostName
	var hn string
	hn, _ = os.Hostname()
	if hn == "" {
		hn = c.IPAddr
	}
	if hn == "" {
		hn = uuid.New().String()
	}

	if c.Port <= 0 {
		c.HostName = fmt.Sprintf("%s:%s", hn, c.App)
	} else {
		c.HostName = fmt.Sprintf("%s:%s:%d", hn, c.App, c.Port)
	}
}
