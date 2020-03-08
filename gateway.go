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
	gatewayNode struct {
		Key  string
		Host string
		Port int
	}
	gatewayModule struct {
		mutex        sync.Mutex
		serviceNodes map[string]gatewayNode
		websiteNodes map[string]gatewayNode
	}
)

func newGateway() *gatewayModule {
	gateway := &gatewayModule{
		serviceNodes: make(map[string]gatewayNode),
		websiteNodes: make(map[string]gatewayNode),
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

	module.serviceNodes[key] = gatewayNode{key, host, port}

	return nil
}

func (module *gatewayModule) Website(host string, port int) error {
	module.mutex.Lock()
	defer module.mutex.Unlock()

	key := fmt.Sprintf("%s:%d", host, port)
	if _, ok := module.websiteNodes[key]; ok {
		return errors.New("已经存在相同的服务节点")
	}

	module.websiteNodes[key] = gatewayNode{key, host, port}

	return nil
}
