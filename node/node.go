package node

import (
	"gitlab.zhonganonline.com/ann/angine"
	acfg "gitlab.zhonganonline.com/ann/angine/config"
	cfg "gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
)

// Node include angine
type Node struct {
	angine *angine.Angine
}

// NodeNum for angine prevote
const (
	NodeNum  = 4
	confPath = "/home/vagrant/gohome/src/angine-demo/build"
)

var conf *cfg.MapConfig

func init() {
	conf = acfg.GetConfig(confPath)
}

// New node
func New() *Node {
	println("conf2", conf.Get("environment").(string))
	println("conf1", conf.Get("log_path").(string))
	angine.Initialize(&angine.AngineTunes{Conf: conf})
	tune := angine.NewAngine(&angine.AngineTunes{Conf: conf})
	return &Node{angine: tune}
}

// Start node
func (node *Node) Start() {
	node.angine.Start()
}

// Stop an node
func (node *Node) Stop() {
	node.angine.Stop()
}
