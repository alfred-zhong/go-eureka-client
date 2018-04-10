package eureka

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"

	"github.com/hudl/fargo"
	"github.com/pkg/errors"
)

// AppClient 代表一个对应 App 的客户端，用以请求数据
type AppClient struct {
	e *fargo.EurekaConnection

	insMapM sync.RWMutex
	insIDs  []string
	insMap  map[string]*fargo.Instance

	App string
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
		return fmt.Sprintf("https://%s:%d", i.IPAddr, i.SecurePort), nil
	}

	if i.PortEnabled {
		return fmt.Sprintf("http://%s:%d", i.IPAddr, i.Port), nil
	}

	return "", errors.Errorf("get instance: %s without url", generateInstanceID(i))
}
