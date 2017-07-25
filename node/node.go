package node

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"gitlab.zhonganonline.com/ann/angine"
	"gitlab.zhonganonline.com/ann/angine/types"
	cmn "gitlab.zhonganonline.com/ann/ann-module/lib/go-common"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	cfg "gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-crypto"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-p2p"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-wire"
	"gitlab.zhonganonline.com/ann/civilwar/src/chain/version"
)

type Node struct {
	MainChainID   string
	running       int64
	config        cfg.Config
	privValidator *types.PrivValidator
	nodeInfo      *p2p.NodeInfo

	logger *zap.Logger

	Angine      *angine.Angine
	AngineTune  *angine.AngineTunes
	Application types.Application
	GenesisDoc  *types.GenesisDoc
}

// Initfiles new config files if not exist
func Initfiles(conf *config.MapConfig) {
	angine.Initialize(&angine.AngineTunes{Conf: conf})
}

// New implement node struct
func New(logger *zap.Logger, config cfg.Config) *Node {
	conf := config.(*cfg.MapConfig)
	tune := &angine.AngineTunes{Conf: conf}
	mainAngine := angine.NewAngine(tune)
	shardApp := NewShardingApp(logger, conf)
	publicKey := mainAngine.PrivValidator().PubKey.(crypto.PubKeyEd25519)
	mainAngine.ConnectApp(shardApp)

	node := &Node{
		MainChainID:   mainAngine.Genesis().ChainID,
		config:        conf,
		privValidator: mainAngine.PrivValidator(),
		nodeInfo:      makeNodeInfo(conf, publicKey, mainAngine.P2PHost(), mainAngine.P2PPort()),
		logger:        logger,
		Angine:        mainAngine,
		AngineTune:    tune,
		Application:   shardApp,
		GenesisDoc:    mainAngine.Genesis(),
	}

	mainAngine.RegisterNodeInfo(node.nodeInfo)

	//	shardApp.setNode(node)

	return node
}

// Run sync rest server
func (node *Node) Run() {
	config := node.config

	if err := node.start(); err != nil {
		cmn.Exit(cmn.Fmt("Failed to start node: %v", err))
	}

	// if config.GetString("rpc_laddr") != "" {
	// 	if _, err := node.StartRPC(); err != nil {
	// 		cmn.PanicCrisis(err)
	// 	}
	// }

	if config.GetBool("pprof") {
		go func() {
			http.ListenAndServe(":6060", nil)
		}()
	}

	fmt.Printf("node (%s) is running on %s:%d ......\n", node.MainChainID, node.nodeInfo.ListenHost(), node.nodeInfo.ListenPort())

	cmn.TrapSignal(func() {
		node.stop()
	})
}

// start adding the listeners.
func (n *Node) start() error {
	if atomic.CompareAndSwapInt64(&n.running, 0, 1) {
		n.Application.Start()
		return n.Angine.Start()
	}
	return fmt.Errorf("already started")
}

func (n *Node) stop() {
	n.logger.Info("Stopping Node")
	n.Application.Stop()
	n.Angine.Stop()
}

func makeNodeInfo(config cfg.Config, pubkey crypto.PubKeyEd25519, p2pHost string, p2pPort uint16) *p2p.NodeInfo {
	nodeInfo := &p2p.NodeInfo{
		PubKey:      pubkey,
		Moniker:     config.GetString("moniker"),
		Network:     config.GetString("chain_id"),
		SigndPubKey: config.GetString("signbyCA"),
		Version:     version.GetVersion(),
		Other: []string{
			cmn.Fmt("wire_version=%v", wire.Version),
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("node_start_at=%s", strconv.FormatInt(time.Now().Unix(), 10)),
			cmn.Fmt("revision=%s", version.GetCommitVersion()),
		},
		RemoteAddr: config.GetString("rpc_laddr"),
		ListenAddr: cmn.Fmt("%v:%v", p2pHost, p2pPort),
	}

	return nodeInfo
}
