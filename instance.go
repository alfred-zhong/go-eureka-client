package eureka

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gojektech/heimdall"
	"github.com/google/uuid"
	"github.com/hudl/fargo"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
)

func init() {
	logging.SetLevel(logging.WARNING, "fargo")
}

const defaultHeartBeatInterval = 60 * time.Second

// Instance 代表一个简单的 Eureka 实例
type Instance struct {
	e *fargo.EurekaConnection

	HostName string
	App      string

	IPAddr     string
	Port       int
	SecurePort int

	// 心跳发送间隔，默认 60s
	HeartBeatInterval time.Duration

	insOnce sync.Once
	ins     *fargo.Instance

	runM    sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// SetServiceUrls 设置 Eureka Server 的服务地址
func (ins *Instance) SetServiceUrls(serviceUrls ...string) {
	e := fargo.NewConn(serviceUrls...)
	ins.e = &e
}

// Run 启动实例注册。方法会监听系统 SIGINT 和 SIGTERM 信号并从 Eureka Server 上注销。
// 方法不会阻塞，会立即返回。
func (ins *Instance) Run() error {
	ins.runM.RLock()

	// 如果已经运行，提前返回
	if ins.running {
		ins.runM.RUnlock()
		return errors.New("instance is already running")
	}
	ins.runM.RUnlock()

	// 开始运行
	ins.runM.Lock()
	ins.running = true
	ins.stopCh = make(chan struct{})

	fargoIns := ins.getFargoInstance()
	// 注册实例
	if err := ins.e.RegisterInstance(fargoIns); err != nil {
		ins.running = false
		ins.runM.Unlock()
		return errors.Wrapf(err, "register to %v fail", ins.e.ServiceUrls)
	}
	ins.runM.Unlock()

	go ins.run(ins.e)

	return nil
}

func (ins *Instance) run(e *fargo.EurekaConnection) {
	// 发送心跳
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
	HeartBeatLoop:
		for {
			select {
			case <-ctx.Done():
				break HeartBeatLoop
			default:
				e.HeartBeatInstance(ins.ins)
			}

			interval := ins.HeartBeatInterval
			if interval <= 0 {
				interval = defaultHeartBeatInterval
			}
			// fmt.Printf("sleep interval: %f\n", interval.Seconds())
			time.Sleep(interval)
		}
	}()

	cleanFunc := func() {
		// 注销实例，取消心跳
		e.DeregisterInstance(ins.ins)
		cancel()
		ins.running = false
	}

	go ins.listenSystemTerm(cleanFunc)

	select {
	case <-ins.stopCh:
		cleanFunc()
	}
}

// 监听操作系统停止信号
func (ins *Instance) listenSystemTerm(cleanFunc func()) {
	exitCh := make(chan os.Signal)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ins.stopCh:
		break
	case <-exitCh:
		cleanFunc()
		os.Exit(0)
	}
}

// Stop 停止实例
func (ins *Instance) Stop() error {
	ins.runM.RLock()
	defer ins.runM.RUnlock()

	if !ins.running {
		return errors.New("instance is not running")
	}

	close(ins.stopCh)
	for ins.running {
	}
	return nil
}

// IsRunning 返回实例是否正处于运行
func (ins *Instance) IsRunning() bool {
	ins.runM.RLock()
	defer ins.runM.RUnlock()

	return ins.running
}

// 获取 fargo.Instance 实例
func (ins *Instance) getFargoInstance() *fargo.Instance {
	ins.insOnce.Do(func() {
		ins.ins = &fargo.Instance{
			HostName:         ins.HostName,
			App:              ins.App,
			IPAddr:           ins.IPAddr,
			VipAddress:       ins.App,
			SecureVipAddress: ins.App,
			DataCenterInfo:   fargo.DataCenterInfo{Name: fargo.MyOwn},
			Status:           fargo.UP,
		}

		if ins.Port > 0 {
			ins.ins.Port = ins.Port
			ins.ins.PortEnabled = true
		}
		if ins.SecurePort > 0 {
			ins.ins.SecurePort = ins.SecurePort
			ins.ins.SecurePortEnabled = true
		}

		// 填充 homePageUrl
		if ins.ins.SecurePortEnabled {
			ins.ins.HomePageUrl = fmt.Sprintf("https://%s:%d/", ins.ins.IPAddr, ins.SecurePort)
		} else if ins.ins.PortEnabled {
			ins.ins.HomePageUrl = fmt.Sprintf("http://%s:%d/", ins.ins.IPAddr, ins.Port)
		}

		// TODO: 填充 healthCheckUrl 和 statusPageUrl
		// ins.ins.HealthCheckUrl = ins.ins.HomePageUrl + "/health"
		// ins.ins.StatusPageUrl = ins.ins.HomePageUrl + "/info"
	})

	return ins.ins
}

// NewInstance 新建一个实例
func NewInstance(app string, serviceUrls ...string) (*Instance, error) {
	return NewInstanceWithPort(app, 0, serviceUrls...)
}

// NewInstanceWithPort 新建一个带有 HTTP 端口的实例
func NewInstanceWithPort(app string, port int, serviceUrls ...string) (*Instance, error) {
	if app == "" {
		return nil, errors.New("app can not be empty")
	}

	if len(serviceUrls) == 0 {
		return nil, errors.New("serviceUrls can not be empty")
	}

	conn := fargo.NewConn(serviceUrls...)
	ins := &Instance{
		e:    &conn,
		App:  app,
		Port: port,
	}
	ins.autoFill()
	return ins, nil
}

// NewInstanceWithSecurePort 新建一个带有 HTTPS 端口的实例
func NewInstanceWithSecurePort(app string, securePort int, serviceUrls ...string) (*Instance, error) {
	if app == "" {
		return nil, errors.New("app can not be empty")
	}

	if len(serviceUrls) == 0 {
		return nil, errors.New("serviceUrls can not be empty")
	}

	conn := fargo.NewConn(serviceUrls...)
	ins := &Instance{
		e:          &conn,
		App:        app,
		SecurePort: securePort,
	}
	ins.autoFill()
	return ins, nil
}

func (ins *Instance) autoFill() {
	// 尝试自动填充 IP
	if ip, ok := AutoIP4(); ok {
		ins.IPAddr = ip
	}

	// 尝试自动填充 HostName
	var hn string
	hn, _ = os.Hostname()
	if hn == "" {
		hn = ins.IPAddr
	}
	if hn == "" {
		hn = uuid.New().String()
	}

	ins.HostName = hn
}

// GetAppClient 根据 App 名称获取 AppClient
func (ins *Instance) GetAppClient(app string) *AppClient {
	return ins.GetAppClientWithTimeout(app, 0)
}

// GetAppClientWithTimeout 根据 App 名称获取带有超时的 AppClient
func (ins *Instance) GetAppClientWithTimeout(app string, timeout time.Duration) *AppClient {
	return &AppClient{
		e:      ins.e,
		App:    app,
		client: heimdall.NewHTTPClient(timeout),
	}
}
