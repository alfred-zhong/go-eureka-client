package eureka

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hudl/fargo"
	"github.com/pkg/errors"
)

// Client 代表一个简单的 Eureka Client
type Client struct {
	HostName string
	App      string

	IPAddr string
	Port   int

	insOnce  sync.Once
	instance *fargo.Instance

	runM    sync.Mutex
	running bool
	stopCh  chan struct{}
}

// Run 启动客户端注册
func (c *Client) Run(address ...string) error {
	c.runM.Lock()
	defer c.runM.Unlock()

	if c.running {
		return errors.New("client is already running")
	}

	go c.run(address...)

	return nil
}

func (c *Client) run(address ...string) {
	c.running = true
	c.stopCh = make(chan struct{})

	instance := c.getInstance()
	e := fargo.NewConn(address...)
	// 注册实例
	if err := e.RegisterInstance(instance); err != nil {
		c.running = false
		return
	}

	// 发送心跳
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
	HeartBeatLoop:
		for {
			select {
			case <-ctx.Done():
				break HeartBeatLoop
			default:
				e.HeartBeatInstance(instance)
			}

			time.Sleep(10 * time.Second)
		}
	}()

	cleanFunc := func() {
		// 注销实例，取消心跳
		e.DeregisterInstance(instance)
		cancel()
		c.running = false
	}

	go c.listenSystemTerm(cleanFunc)

	select {
	case <-c.stopCh:
		cleanFunc()
	}
}

func (c *Client) listenSystemTerm(cleanFunc func()) {
	exitCh := make(chan os.Signal)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	select {
	case <-c.stopCh:
		break
	case <-exitCh:
		cleanFunc()
	}
}

// Stop 停止客户端
func (c *Client) Stop() error {
	c.runM.Lock()
	defer c.runM.Unlock()

	if !c.running {
		return errors.New("client is not running")
	}

	close(c.stopCh)
	c.running = false
	return nil
}

// 获取 eureka instance 实例
func (c *Client) getInstance() *fargo.Instance {
	c.insOnce.Do(func() {
		c.instance = &fargo.Instance{
			HostName:         c.HostName,
			App:              c.App,
			IPAddr:           c.IPAddr,
			VipAddress:       c.IPAddr,
			DataCenterInfo:   fargo.DataCenterInfo{Name: fargo.MyOwn},
			SecureVipAddress: c.IPAddr,
			Status:           fargo.UP,
		}

		if c.Port > 0 {
			c.instance.Port = c.Port
			c.instance.PortEnabled = true
		}
	})

	return c.instance
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
