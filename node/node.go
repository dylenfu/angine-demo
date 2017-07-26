package node

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"gitlab.zhonganonline.com/ann/angine"
	"gitlab.zhonganonline.com/ann/angine/types"
	cmn "gitlab.zhonganonline.com/ann/ann-module/lib/go-common"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	cfg "gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-crypto"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-p2p"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-rpc/server"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-wire"
	"gitlab.zhonganonline.com/ann/civilwar/src/chain/version"
)

const (
	ReceiptsPrefix  = "receipts-"
	OfficialAddress = "0x7752b42608a0f1943c19fc5802cb027e60b4c911"
)

var Apps = make(map[string]types.AppMaker)

type Node struct {
	MainChainID string
	MainShard   *ShardNode

	config        cfg.Config
	privValidator *types.PrivValidator
	nodeInfo      *p2p.NodeInfo

	logger *zap.Logger
}

func AppExists(name string) (yes bool) {
	_, yes = Apps[name]
	return
}

func NewNode(logger *zap.Logger, config cfg.Config) *Node {
	conf := config.(*cfg.MapConfig)
	shardApp := NewShardingApp(logger, conf)
	tune := &angine.AngineTunes{Conf: conf}
	mainAngine := angine.NewAngine(tune)
	mainAngine.ConnectApp(shardApp)

	node := &Node{
		MainChainID: mainAngine.Genesis().ChainID,
		MainShard: &ShardNode{
			Application: shardApp,
			Angine:      mainAngine,
			AngineTune:  tune,
			GenesisDoc:  mainAngine.Genesis(),
		},

		nodeInfo:      makeNodeInfo(conf, mainAngine.PrivValidator().PubKey.(crypto.PubKeyEd25519), mainAngine.P2PHost(), mainAngine.P2PPort()),
		config:        conf,
		privValidator: mainAngine.PrivValidator(),
		logger:        logger,
	}

	mainAngine.RegisterNodeInfo(node.nodeInfo)
	shardApp.setNode(node)

	return node
}

func RunNode(logger *zap.Logger, config cfg.Config) {
	node := NewNode(logger, config)
	if err := node.Start(); err != nil {
		cmn.Exit(cmn.Fmt("Failed to start node: %v", err))
	}
	if config.GetString("rpc_laddr") != "" {
		if _, err := node.StartRPC(); err != nil {
			cmn.PanicCrisis(err)
		}
	}
	if config.GetBool("pprof") {
		go func() {
			http.ListenAndServe(":6060", nil)
		}()
	}

	fmt.Printf("node (%s) is running on %s:%d ......\n", node.MainChainID, node.NodeInfo().ListenHost(), node.NodeInfo().ListenPort())

	cmn.TrapSignal(func() {
		node.Stop()
	})
}

// Call Start() after adding the listeners.
func (n *Node) Start() error {
	if err := n.MainShard.Start(); err != nil {
		return fmt.Errorf("fail to start, error: %v", err)
	}

	// restore will take charge of restarting all shards
	n.MainShard.Application.(*ShardingApp).Start()

	return nil
}

func (n *Node) Stop() {
	n.logger.Info("Stopping Node")
	n.MainShard.Application.(*ShardingApp).Stop()
	n.MainShard.Stop()
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
			// Fmt("consensus_version=%v", n.StateMachine.Version()),
			// Fmt("rpc_version=%v/%v", rpc.Version, rpccore.Version),
			cmn.Fmt("node_start_at=%s", strconv.FormatInt(time.Now().Unix(), 10)),
			cmn.Fmt("revision=%s", version.GetCommitVersion()),
		},
		RemoteAddr: config.GetString("rpc_laddr"),
		ListenAddr: cmn.Fmt("%v:%v", p2pHost, p2pPort),
	}

	// We assume that the rpcListener has the same ExternalAddress.
	// This is probably true because both P2P and RPC listeners use UPnP,
	// except of course if the rpc is only bound to localhost

	return nodeInfo
}

func (n *Node) NodeInfo() *p2p.NodeInfo {
	return n.nodeInfo
}

func (n *Node) StartRPC() ([]net.Listener, error) {
	listenAddrs := strings.Split(n.config.GetString("rpc_laddr"), ",")
	listeners := make([]net.Listener, len(listenAddrs))

	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		// wm := rpcserver.NewWebsocketManager(rpcRoutes, n.evsw)
		// mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(n.logger, mux, n.rpcRoutes())
		listener, err := rpcserver.StartHTTPServer(n.logger, listenAddr, mux)
		if err != nil {
			return nil, err
		}
		listeners[i] = listener
	}

	return listeners, nil
}

func (n *Node) PrivValidator() *types.PrivValidator {
	return n.privValidator
}

func Initfiles(conf *config.MapConfig) {
	angine.Initialize(&angine.AngineTunes{Conf: conf})
}
