package node

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	types "gitlab.zhonganonline.com/ann/angine/types"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-crypto"
	rpc "gitlab.zhonganonline.com/ann/ann-module/lib/go-rpc/server"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-wire"
	"gitlab.zhonganonline.com/ann/civilwar/src/chain/version"
)

const ChainIDArg = "chainid"

// RPCNode define the node's abilities provided for rpc calls
type RPCNode interface {
	GetShard(string) (*ShardNode, error)
	Height() int
	GetBlock(height int) (*types.Block, *types.BlockMeta)
	BroadcastTx(tx []byte) error
	BroadcastTxCommit(tx []byte) error
	FlushMempool()
	GetValidators() (int, []*types.Validator)
	GetP2PNetInfo() (bool, []string, []*types.Peer)
	GetNumPeers() int
	GetConsensusStateInfo() (string, []string)
	GetNumUnconfirmedTxs() int
	GetUnconfirmedTxs() []types.Tx
	IsNodeValidator(pub crypto.PubKey) bool
	GetBlacklist() []string
}

type rpcHandler struct {
	node *Node
}

var (
	ErrInvalidChainID = fmt.Errorf("no such chain id")
)

func newRPCHandler(n *Node) *rpcHandler {
	return &rpcHandler{node: n}
}

func (n *Node) rpcRoutes() map[string]*rpc.RPCFunc {
	h := newRPCHandler(n)
	return map[string]*rpc.RPCFunc{
		// subscribe/unsubscribe are reserved for websocket events.
		// "subscribe":   rpc.NewWSRPCFunc(SubscribeResult, argsWithChainID("event")),
		// "unsubscribe": rpc.NewWSRPCFunc(UnsubscribeResult, argsWithChainID("event")),

		// info API
		"shards":               rpc.NewRPCFunc(h.Shards, ""),
		"status":               rpc.NewRPCFunc(h.Status, argsWithChainID("")),
		"net_info":             rpc.NewRPCFunc(h.NetInfo, argsWithChainID("")),
		"blockchain":           rpc.NewRPCFunc(h.BlockchainInfo, argsWithChainID("minHeight,maxHeight")),
		"genesis":              rpc.NewRPCFunc(h.Genesis, argsWithChainID("")),
		"block":                rpc.NewRPCFunc(h.Block, argsWithChainID("height")),
		"validators":           rpc.NewRPCFunc(h.Validators, argsWithChainID("")),
		"dump_consensus_state": rpc.NewRPCFunc(h.DumpConsensusState, argsWithChainID("")),
		"unconfirmed_txs":      rpc.NewRPCFunc(h.UnconfirmedTxs, argsWithChainID("")),
		"num_unconfirmed_txs":  rpc.NewRPCFunc(h.NumUnconfirmedTxs, argsWithChainID("")),
		"za_surveillance":      rpc.NewRPCFunc(h.ZaSurveillance, argsWithChainID("")),
		"core_version":         rpc.NewRPCFunc(h.CoreVersion, argsWithChainID("")),

		// broadcast API
		"broadcast_tx_commit": rpc.NewRPCFunc(h.BroadcastTxCommit, argsWithChainID("tx")),
		"broadcast_tx_sync":   rpc.NewRPCFunc(h.BroadcastTx, argsWithChainID("tx")),

		// query API
		"query": rpc.NewRPCFunc(h.Query, argsWithChainID("query")),
		"info":  rpc.NewRPCFunc(h.Info, argsWithChainID("")),

		// control API
		// "dial_seeds":           rpc.NewRPCFunc(h.UnsafeDialSeeds, argsWithChainID("seeds")),
		"unsafe_flush_mempool": rpc.NewRPCFunc(h.UnsafeFlushMempool, argsWithChainID("")),
		// "unsafe_set_config":    rpc.NewRPCFunc(h.UnsafeSetConfig, argsWithChainID("type,key,value")),

		// profiler API
		// "unsafe_start_cpu_profiler": rpc.NewRPCFunc(UnsafeStartCPUProfilerResult, argsWithChainID("filename")),
		// "unsafe_stop_cpu_profiler":  rpc.NewRPCFunc(UnsafeStopCPUProfilerResult, argsWithChainID("")),
		// "unsafe_write_heap_profile": rpc.NewRPCFunc(UnsafeWriteHeapProfileResult, argsWithChainID("filename")),

		// specialOP API
		"request_special_op": rpc.NewRPCFunc(h.RequestSpecialOP, argsWithChainID("tx")),
		"vote_special_op":    rpc.NewRPCFunc(h.VoteSpecialOP, argsWithChainID("tx")),
		// "request_vote_channel": rpc.NewRPCFunc(RequestForVoteChannel, "tx"),

		// refuse_list API
		"blacklist": rpc.NewRPCFunc(h.Blacklist, argsWithChainID("")),

		// sharding API
		// "shard_join": rpc.NewRPCFunc(h.ShardJoin, "gdata,cdata,sig"),

		// get account api
		"get_account": rpc.NewRPCFunc(h.getAccount, argsWithChainID("address")),
	}
}

type AccountResult struct {
	Amount int `json:"amount"`
}

func (h *rpcHandler) getAccount(chainId string, address string) (AccountResult, error) {
	var res = 0
	if amount, ok := managedState.accounts[address]; ok {
		res = amount
	} else {
		res = 0
	}
	return AccountResult{Amount: res}, nil
}

func (h *rpcHandler) getShard(chainID string) (*ShardNode, error) {
	var shard *ShardNode
	var err error
	if chainID == h.node.MainChainID {
		shard = h.node.MainShard
	} else {
		app := h.node.MainShard.Application.(*ShardingApp)
		shard, err = app.GetShard(chainID)
		if err != nil {
			return nil, ErrInvalidChainID
		}
	}

	return shard, nil
}

func (h *rpcHandler) Shards() (types.RPCResult, error) {
	app := h.node.MainShard.Application.(*ShardingApp)
	app.Lock()
	defer app.Unlock()
	names := make([]string, 0, len(app.Shards))
	for n := range app.Shards {
		names = append(names, n)
	}
	return &types.ResultShards{Names: names}, nil
}

func (h *rpcHandler) Status(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	var (
		latestBlockMeta *types.BlockMeta
		latestBlockHash []byte
		latestAppHash   []byte
		latestBlockTime int64
	)
	latestHeight := shard.Angine.Height()
	if latestHeight != 0 {
		_, latestBlockMeta = shard.Angine.GetBlock(latestHeight)
		latestBlockHash = latestBlockMeta.Hash
		latestAppHash = latestBlockMeta.Header.AppHash
		latestBlockTime = latestBlockMeta.Header.Time.UnixNano()
	}

	return &types.ResultStatus{
		NodeInfo:          shard.Angine.GetNodeInfo(),
		PubKey:            shard.Angine.PrivValidator().PubKey,
		LatestBlockHash:   latestBlockHash,
		LatestAppHash:     latestAppHash,
		LatestBlockHeight: latestHeight,
		LatestBlockTime:   latestBlockTime}, nil
}

func (h *rpcHandler) Genesis(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	return &types.ResultGenesis{Genesis: shard.GenesisDoc}, nil
}

func (h *rpcHandler) Block(chainID string, height int) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	if height == 0 {
		return nil, fmt.Errorf("height must be greater than 0")
	}
	if height > shard.Angine.Height() {
		return nil, fmt.Errorf("height must be less than the current blockchain height")
	}
	res := types.ResultBlock{}
	res.Block, res.BlockMeta = shard.Angine.GetBlock(height)
	return &res, nil
}

func (h *rpcHandler) BlockchainInfo(chainID string, minHeight, maxHeight int) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	if minHeight > maxHeight {
		return nil, fmt.Errorf("maxHeight has to be bigger than minHeight")
	}

	blockStoreHeight := shard.Angine.Height()
	if maxHeight == 0 {
		maxHeight = blockStoreHeight
	} else if blockStoreHeight < maxHeight {
		maxHeight = blockStoreHeight
	}
	if minHeight == 0 {
		if maxHeight-20 > 1 {
			minHeight = maxHeight - 20
		} else {
			minHeight = 1
		}
	}
	blockMetas := []*types.BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		_, blockMeta := shard.Angine.GetBlock(height)
		blockMetas = append(blockMetas, blockMeta)
	}
	return &types.ResultBlockchainInfo{LastHeight: blockStoreHeight, BlockMetas: blockMetas}, nil
}

func (h *rpcHandler) DumpConsensusState(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	res := types.ResultDumpConsensusState{}
	res.RoundState, res.PeerRoundStates = shard.Angine.GetConsensusStateInfo()
	return &res, nil
}

func (h *rpcHandler) UnconfirmedTxs(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	res := types.ResultUnconfirmedTxs{}
	res.Txs = shard.Angine.GetUnconfirmedTxs()
	res.N = len(res.Txs)
	return &res, nil
}

func (h *rpcHandler) NumUnconfirmedTxs(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	return &types.ResultUnconfirmedTxs{N: shard.Angine.GetNumUnconfirmedTxs(), Txs: nil}, nil
}

func (h *rpcHandler) UnsafeFlushMempool(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	shard.Angine.FlushMempool()
	return &types.ResultUnsafeFlushMempool{}, nil
}

func (h *rpcHandler) BroadcastTx(chainID string, tx []byte) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	if err := shard.Application.CheckTx(tx); err != nil {
		return nil, err
	}
	if err := shard.Angine.BroadcastTx(tx); err != nil {
		return nil, err
	}
	return &types.ResultBroadcastTx{Code: 0}, nil
}

func (h *rpcHandler) BroadcastTxCommit(chainID string, tx []byte) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	if err := shard.Application.CheckTx(tx); err != nil {
		return nil, err
	}
	if err := shard.Angine.BroadcastTxCommit(tx); err != nil {
		return nil, err
	}

	return &types.ResultBroadcastTxCommit{Code: 0}, nil
}

func (h *rpcHandler) Query(chainID string, query []byte) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	return &types.ResultQuery{Result: shard.Application.Query(query)}, nil
}

func (h *rpcHandler) Info(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	res := shard.Application.Info()
	return &res, nil
}

func (h *rpcHandler) Validators(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	height, vs := shard.Angine.GetValidators()
	return &types.ResultValidators{
		Validators:  vs,
		BlockHeight: height,
	}, nil
}

func (h *rpcHandler) CoreVersion(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	appInfo := shard.Application.Info()
	vs := strings.Split(version.GetCommitVersion(), "-")
	res := types.ResultCoreVersion{
		Version:    vs[0],
		AppName:    version.AppName(),
		AppVersion: appInfo.Version,
	}
	if len(vs) > 1 {
		res.Hash = vs[1]
	}

	return &res, nil
}

func (h *rpcHandler) ZaSurveillance(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	bcHeight := shard.Angine.Height()

	var totalNumTxs, txAvg int64
	if bcHeight >= 2 {
		startHeight := bcHeight - 200
		if startHeight < 1 {
			startHeight = 1
		}
		eBlock, _ := shard.Angine.GetBlock(bcHeight)
		endTime := eBlock.Header.Time
		sBlock, _ := shard.Angine.GetBlock(startHeight)
		startTime := sBlock.Header.Time
		totalNumTxs += int64(sBlock.Header.NumTxs)
		dura := endTime.Sub(startTime)
		for h := startHeight + 1; h < bcHeight; h++ {
			block, _ := shard.Angine.GetBlock(h)
			totalNumTxs += int64(block.Header.NumTxs)
		}
		if totalNumTxs > 0 {
			txAvg = int64(dura) / totalNumTxs
		}
	}

	var runningTime time.Duration
	for _, oth := range h.node.NodeInfo().Other {
		if strings.HasPrefix(oth, "node_start_at") {
			ts, err := strconv.ParseInt(string(oth[14:]), 10, 64)
			if err != nil {
				return -1, err
			}
			runningTime = time.Duration(time.Now().Unix() - ts)
		}
	}

	_, vals := shard.Angine.GetValidators()

	res := types.ResultSurveillance{
		Height:        bcHeight,
		NanoSecsPerTx: time.Duration(txAvg),
		Addr:          h.node.NodeInfo().RemoteAddr,
		IsValidator:   shard.Angine.IsNodeValidator(h.node.NodeInfo().PubKey),
		NumValidators: len(vals),
		NumPeers:      shard.Angine.GetNumPeers(),
		RunningTime:   runningTime,
		PubKey:        h.node.NodeInfo().PubKey.KeyString(),
	}
	return &res, nil
}

func (h *rpcHandler) NetInfo(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	res := types.ResultNetInfo{}
	res.Listening, res.Listeners, res.Peers = shard.Angine.GetP2PNetInfo()
	return &res, nil
}

func (h *rpcHandler) Blacklist(chainID string) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	return &types.ResultRefuseList{Result: shard.Angine.GetBlacklist()}, nil
}

func (h *rpcHandler) RequestSpecialOP(chainID string, tx []byte) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}

	if err := shard.Angine.ProcessSpecialOP(tx); err != nil {
		res := &types.ResultRequestSpecialOP{
			Code: types.CodeType_InternalError,
			Log:  err.Error(),
		}
		return res, err
	}

	return &types.ResultRequestSpecialOP{
		Code: types.CodeType_OK,
	}, nil
}

func (h *rpcHandler) VoteSpecialOP(chainID string, tx []byte) (types.RPCResult, error) {
	shard, err := h.getShard(chainID)
	if err != nil {
		return nil, ErrInvalidChainID
	}
	if !types.IsSpecialOP(tx) {
		return nil, fmt.Errorf("not a specialop")
	}

	cmd := new(types.SpecialOPCmd)
	if err = wire.ReadBinaryBytes(types.UnwrapTx(tx), cmd); err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}
	res, err := shard.Angine.CheckSpecialOp(cmd)
	if err != nil {
		return &types.ResultRequestSpecialOP{
			Code: types.CodeType_InternalError,
			Log:  err.Error(),
		}, err
	}
	return &types.ResultRequestSpecialOP{
		Code: types.CodeType_OK,
		Data: res,
	}, nil
}

// func BroadcastTxCommitResult(chainID string, tx []byte) (rpctypes.TMResult, error) {
//	shard := FindShard(chainID)
//	if err := shard.Application.CheckTx(tx); err != nil {
//		return nil, err
//	}
//	shard.Angine.BroadcastTxCommit(tx)
// }

func argsWithChainID(args string) string {
	if args == "" {
		return ChainIDArg
	}
	return ChainIDArg + "," + args
}
