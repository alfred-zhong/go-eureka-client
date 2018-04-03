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

// 默认心跳为 10s
const defaultHeartBeatSeconds = 10

// Client 代表一个简单的 Eureka Client
type Client struct {
	HostName string
	App      string

	IPAddr string
	Port   int

	insOnce  sync.Once
	instance *fargo.Instance

	runM    sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// Run 启动客户端注册
func (c *Client) Run(address ...string) error {
	c.runM.RLock()

	// 如果已经运行，提前返回
	if c.running {
		c.runM.RUnlock()
		return errors.New("client is already running")
	}
	c.runM.RUnlock()

	// 开始运行
	c.runM.Lock()
	c.running = true
	c.stopCh = make(chan struct{})

	instance := c.getInstance()
	e := fargo.NewConn(address...)
	// 注册实例
	if err := e.RegisterInstance(instance); err != nil {
		c.running = false
		c.runM.Unlock()
		return errors.Wrapf(err, "register to %v fail", address)
	}
	c.runM.Unlock()

	go c.run(&e)

	return nil
}

func (c *Client) run(e *fargo.EurekaConnection) {
	// 发送心跳
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
	HeartBeatLoop:
		for {
			select {
			case <-ctx.Done():
				break HeartBeatLoop
			default:
				e.HeartBeatInstance(c.instance)
			}

			time.Sleep(defaultHeartBeatSeconds * time.Second)
		}
	}()

	cleanFunc := func() {
		// 注销实例，取消心跳
		e.DeregisterInstance(c.instance)
		cancel()
		c.running = false
	}

	go c.listenSystemTerm(cleanFunc)

	select {
	case <-c.stopCh:
		cleanFunc()
	}
}

// 监听操作系统停止信号
func (c *Client) listenSystemTerm(cleanFunc func()) {
	exitCh := make(chan os.Signal)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-c.stopCh:
		break
	case <-exitCh:
		cleanFunc()
		os.Exit(0)
	}
}

// Stop 停止客户端
func (c *Client) Stop() error {
	c.runM.RLock()
	defer c.runM.RUnlock()

	if !c.running {
		return errors.New("client is not running")
	}

	close(c.stopCh)
	for c.running {
	}
	return nil
}

// IsRunning 返回 Client 是否正处于运行
func (c *Client) IsRunning() bool {
	c.runM.RLock()
	defer c.runM.RUnlock()

	return c.running
}

// 获取 eureka instance 实例
func (c *Client) getInstance() *fargo.Instance {
	c.insOnce.Do(func() {
		c.instance = &fargo.Instance{
			HostName:         c.HostName,
			App:              c.App,
			IPAddr:           c.IPAddr,
			VipAddress:       c.App,
			SecureVipAddress: c.App,
			DataCenterInfo:   fargo.DataCenterInfo{Name: fargo.MyOwn},
			Status:           fargo.UP,
		}

		if c.Port > 0 {
			c.instance.Port = c.Port
			c.instance.PortEnabled = true
			c.instance.HomePageUrl = fmt.Sprintf("http://%s:%d/", c.instance.HostName, c.Port)
			// c.instance.HealthCheckUrl = c.instance.HomePageUrl + "/health"
			// c.instance.StatusPageUrl = c.instance.HomePageUrl + "/info"
		}
	})

	return c.instance
}

// NewClient 新建一个 Client
func NewClient(app string) (*Client, error) {
	if app == "" {
		return nil, errors.New("app can not be empty")
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
		return nil, errors.New("app can not be empty")
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

	c.HostName = hn
}
