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
	// Default just print logs above level of WARNING bt fargo.
	logging.SetLevel(logging.WARNING, "fargo")
}

const defaultHeartBeatInterval = 60 * time.Second

// Instance defines a simple eureka instance.
type Instance struct {
	e *fargo.EurekaConnection

	HostName string
	App      string

	IPAddr     string
	Port       int
	SecurePort int

	// Default 60s
	HeartBeatInterval time.Duration

	insOnce sync.Once
	ins     *fargo.Instance

	runM    sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// SetServiceUrls sets the service urls of eureka server.
func (ins *Instance) SetServiceUrls(serviceUrls ...string) {
	e := fargo.NewConn(serviceUrls...)
	ins.e = &e
}

// Run make the instance register to the eureka server. Note that this method
// will not block.
//
// Also it listens for SIGINT and SIGTERM signal. By Receiving them, it will
// deregister ths instance from the eureka server.
func (ins *Instance) Run() error {
	ins.runM.RLock()

	// If already running, returns.
	if ins.running {
		ins.runM.RUnlock()
		return errors.New("instance is already running")
	}
	ins.runM.RUnlock()

	// Running the instance
	ins.runM.Lock()
	ins.running = true
	ins.stopCh = make(chan struct{})

	fargoIns := ins.getFargoInstance()
	// Register the instance to the eureka server.
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
	// Send heartbeat in loop.
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
		// Deregister the instance and cancel the heartbeat sending loop.
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

// Stop the instance when receive SIGINT and SIGTERM signal.
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

// Stop stops the instance. Deregister the instance from the eureka server.
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

// IsRunning tells whether the instance is still running.
func (ins *Instance) IsRunning() bool {
	ins.runM.RLock()
	defer ins.runM.RUnlock()

	return ins.running
}

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

		// handle homePageUrl
		if ins.ins.SecurePortEnabled {
			ins.ins.HomePageUrl = fmt.Sprintf("https://%s:%d/", ins.ins.IPAddr, ins.SecurePort)
		} else if ins.ins.PortEnabled {
			ins.ins.HomePageUrl = fmt.Sprintf("http://%s:%d/", ins.ins.IPAddr, ins.Port)
		}

		// TODO: handle healthCheckUrl and statusPageUrl
		// ins.ins.HealthCheckUrl = ins.ins.HomePageUrl + "/health"
		// ins.ins.StatusPageUrl = ins.ins.HomePageUrl + "/info"
	})

	return ins.ins
}

// NewInstance returns a new Instance with no port enabled.
func NewInstance(app string, serviceUrls ...string) (*Instance, error) {
	return NewInstanceWithPort(app, 0, serviceUrls...)
}

// NewInstanceWithPort returns a new Instance with one port enabled.
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

// NewInstanceWithSecurePort returns a new Instance with one secure port(HTTPS)
// enabled.
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
	// Try to get ip automatically and fill into the IPAddr of instance.
	if ip, ok := AutoIP4(); ok {
		ins.IPAddr = ip
	}

	// Fill in the Hostname of instance.
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

// GetAppClient returns a new AppClient by app name.
func (ins *Instance) GetAppClient(app string) *AppClient {
	return ins.GetAppClientWithTimeout(app, 0)
}

// GetAppClientWithTimeout returns a new AppClient with timeout by app name.
func (ins *Instance) GetAppClientWithTimeout(app string, timeout time.Duration) *AppClient {
	return &AppClient{
		e:      ins.e,
		App:    app,
		client: heimdall.NewHTTPClient(timeout),
	}
}
