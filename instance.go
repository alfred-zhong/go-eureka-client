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
	"github.com/op/go-logging"
	"github.com/pkg/errors"
)

func init() {
	logging.SetLevel(logging.WARNING, "fargo")
}

// 默认心跳为 10s
const defaultHeartBeatSeconds = 10

// Instance 代表一个简单的 Eureka 实例
type Instance struct {
	HostName string
	App      string

	IPAddr string
	Port   int

	insOnce sync.Once
	ins     *fargo.Instance

	runM    sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// Run 启动实例注册
func (ins *Instance) Run(address ...string) error {
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
	e := fargo.NewConn(address...)
	// 注册实例
	if err := e.RegisterInstance(fargoIns); err != nil {
		ins.running = false
		ins.runM.Unlock()
		return errors.Wrapf(err, "register to %v fail", address)
	}
	ins.runM.Unlock()

	go ins.run(&e)

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

			time.Sleep(defaultHeartBeatSeconds * time.Second)
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
			/* UniqueID: func(i fargo.Instance) string {
				id := i.HostName + ":" + i.App
				if i.PortEnabled {
					id += ":" + strconv.Itoa(i.Port)
				}
				return id
			}, */
		}

		if ins.Port > 0 {
			ins.ins.Port = ins.Port
			ins.ins.PortEnabled = true
			ins.ins.HomePageUrl = fmt.Sprintf("http://%s:%d/", ins.ins.IPAddr, ins.Port)
			// ins.ins.HealthCheckUrl = ins.ins.HomePageUrl + "/health"
			// ins.ins.StatusPageUrl = ins.ins.HomePageUrl + "/info"
		}
	})

	return ins.ins
}

// NewInstance 新建一个实例
func NewInstance(app string) (*Instance, error) {
	if app == "" {
		return nil, errors.New("app can not be empty")
	}

	ins := &Instance{
		App: app,
	}
	ins.autoFill()
	return ins, nil
}

// NewInstanceWithPort 新建一个带有端口的实例
func NewInstanceWithPort(app string, port int) (*Instance, error) {
	if app == "" {
		return nil, errors.New("app can not be empty")
	}

	ins := &Instance{
		App:  app,
		Port: port,
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
