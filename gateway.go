package ark

import (
	"errors"
	"fmt"
	"sync"
)

type (
	GatewayConfig struct {
		Driver string `toml:"driver"`
	}
	serviceNode struct {
		Key  string
		Host string
		Port int
	}
	websiteNode struct {
		Key  string
		Host string
		Port int
	}
	gatewayModule struct {
		mutex        sync.Mutex
		serviceNodes map[string]serviceNode
		websiteNodes map[string]websiteNode
	}
)

func newGateway() *gatewayModule {
	gateway := &gatewayModule{
		serviceNodes: make(map[string]serviceNode),
		websiteNodes: make(map[string]websiteNode),
	}
	return gateway
}

func (module *gatewayModule) Service(host string, port int) error {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	key := fmt.Sprintf("%s:%d", host, port)
	if _, ok := module.serviceNodes[key]; ok {
		return errors.New("已经存在相同的服务节点")
	}

	module.serviceNodes[key] = serviceNode{key, host, port}

	return nil
}

func (module *gatewayModule) Website(host string, port int) error {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	key := fmt.Sprintf("%s:%d", host, port)
	if _, ok := module.websiteNodes[key]; ok {
		return errors.New("已经存在相同的服务节点")
	}

	module.websiteNodes[key] = websiteNode{key, host, port}

	return nil
}
