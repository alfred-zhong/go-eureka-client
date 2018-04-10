package eureka

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gojektech/heimdall"
	"github.com/hudl/fargo"
	"github.com/pkg/errors"
)

// AppClient 代表一个对应 App 的客户端，用以请求数据
type AppClient struct {
	e *fargo.EurekaConnection

	App string

	insMapM sync.RWMutex
	insIDs  []string
	insMap  map[string]*fargo.Instance

	client heimdall.Client
}

// NewAppClient 新建一个 AppClient
func NewAppClient(app string, serviceUrls ...string) (*AppClient, error) {
	return NewAppClientWithTimeout(app, 0, serviceUrls...)
}

// NewAppClientWithTimeout 新建一个指定超时的 AppClient
func NewAppClientWithTimeout(app string, timeout time.Duration, serviceUrls ...string) (*AppClient, error) {
	if app == "" {
		return nil, errors.New("app can not be empty")
	}

	if len(serviceUrls) == 0 {
		return nil, errors.New("serviceUrls can not be empty")
	}

	conn := fargo.NewConn(serviceUrls...)
	return &AppClient{
		e:      &conn,
		App:    app,
		client: heimdall.NewHTTPClient(timeout),
	}, nil
}

// GetURL 随机获取当前 App 的某个 Instance 的 URL
func (ac *AppClient) GetURL() (string, error) {
	ac.insMapM.RLock()
	insMapLen := len(ac.insIDs)
	ac.insMapM.RUnlock()

	if insMapLen == 0 {
		if err := ac.RefreshInstance(); err != nil {
			return "", errors.Wrap(err, "refresh instance fail")
		}
	}

	ins, err := ac.chooseInstance()
	if err != nil {
		return "", err
	}

	return getInstanceURL(ins)
}

func (ac *AppClient) chooseInstance() (*fargo.Instance, error) {
	ac.insMapM.RLock()
	defer ac.insMapM.RUnlock()

	if len(ac.insIDs) == 0 {
		return nil, errors.Errorf("no instance found for app: %s", ac.App)
	}

	id := ac.insIDs[rand.Int()%len(ac.insIDs)]
	return ac.insMap[id], nil
}

// RefreshInstance 刷新当前 App 的 Instance 列表
func (ac *AppClient) RefreshInstance() error {
	if ac.App == "" {
		return errors.New("app is empty, can not refresh")
	}

	app, err := ac.e.GetApp(ac.App)
	if err != nil {
		return errors.Wrapf(err, "get app: %s from eureka: %v fail", ac.App, ac.e.ServiceUrls)
	}

	ac.insMapM.Lock()
	defer ac.insMapM.Unlock()

	ac.insIDs = make([]string, 0, len(app.Instances))
	ac.insMap = make(map[string]*fargo.Instance)
	for i := range app.Instances {
		insID := generateInstanceID(app.Instances[i])
		ac.insIDs = append(ac.insIDs, insID)
		ac.insMap[insID] = app.Instances[i]
	}
	return nil
}

func generateInstanceID(i *fargo.Instance) string {
	id := i.HostName + ":" + i.App
	if i.PortEnabled {
		id += ":" + strconv.Itoa(i.Port)
	} else if i.SecurePortEnabled {
		id += ":" + strconv.Itoa(i.SecurePort)
	}
	return id
}

func getInstanceURL(i *fargo.Instance) (string, error) {
	if i == nil {
		return "", errors.New("instance is nil")
	}

	// 优先选择 homePageUrl
	if i.HomePageUrl != "" {
		return i.HomePageUrl, nil
	}

	// homePageUrl 为空时尝试生成 https 或者 http
	if i.SecurePortEnabled {
		return fmt.Sprintf("https://%s:%d/", i.IPAddr, i.SecurePort), nil
	}

	if i.PortEnabled {
		return fmt.Sprintf("http://%s:%d/", i.IPAddr, i.Port), nil
	}

	return "", errors.Errorf("get instance: %s without url", generateInstanceID(i))
}

// Get 请求
func (ac *AppClient) Get(path string, headers http.Header) (*http.Response, error) {
	url, err := ac.concatURL(path)
	if err != nil {
		return nil, err
	}
	return ac.client.Get(url, headers)
}

// Post 请求
func (ac *AppClient) Post(path string, body io.Reader, headers http.Header) (*http.Response, error) {
	url, err := ac.concatURL(path)
	if err != nil {
		return nil, err
	}
	return ac.Post(url, body, headers)
}

// Put 请求
func (ac *AppClient) Put(path string, body io.Reader, headers http.Header) (*http.Response, error) {
	url, err := ac.concatURL(path)
	if err != nil {
		return nil, err
	}
	return ac.Put(url, body, headers)
}

// Patch 请求
func (ac *AppClient) Patch(path string, body io.Reader, headers http.Header) (*http.Response, error) {
	url, err := ac.concatURL(path)
	if err != nil {
		return nil, err
	}
	return ac.Patch(url, body, headers)
}

// Delete 请求
func (ac *AppClient) Delete(path string, headers http.Header) (*http.Response, error) {
	url, err := ac.concatURL(path)
	if err != nil {
		return nil, err
	}
	return ac.Delete(url, headers)
}

func (ac *AppClient) concatURL(path string) (string, error) {
	baseURL, err := ac.GetURL()
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return baseURL + path, nil
}
