package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"gitlab.zhonganonline.com/ann/angine/types"
	cmn "gitlab.zhonganonline.com/ann/ann-module/lib/go-common"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-crypto"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-db"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-merkle"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-wire"
	acfg "gitlab.zhonganonline.com/ann/civilwar/src/chain/config"
)

const (
	ShardTag = "shrd"

	ShardRegister byte = iota
	ShardCreate
	ShardJoin
	ShardLeave
	ShardDelete
)

type LastBlockInfo struct {
	Height uint64
	Txs    [][]byte
	Hash   []byte
}

type ShardingApp struct {
	config      config.Config
	privkey     crypto.PrivKeyEd25519
	chainDb     *db.GoLevelDB
	node        *Node
	engineHooks types.Hooks
	logger      *zap.Logger

	mtx sync.Mutex

	Txs             [][]byte
	RunningChainIDs map[string]string
	Shards          map[string]*ShardNode
}

type ManagedState struct {
	accounts map[string]int
}

var managedState = ManagedState{accounts: make(map[string]int)}

type ShardingTx struct {
	App           string                  `json:"app"`
	Act           byte                    `json:"act"`
	ChainID       string                  `json:"chainid"`
	Genesis       types.GenesisDoc        `json:"genesis"`
	Config        map[string]interface{}  `json:"config"`
	Time          time.Time               `json:"time"`
	Signature     crypto.SignatureEd25519 `json:"signature"`
	DestAddress   string                  `json:"destaddress"`
	SourceAddress string                  `json:"sourceaddress"`
	Amount        int                     `json:"amount"`
}

func (t *ShardingTx) SignByPrivKey(p crypto.PrivKeyEd25519) error {
	txBytes, err := json.Marshal(t)
	if err != nil {
		return err
	}
	t.Signature = p.Sign(txBytes).(crypto.SignatureEd25519)

	return nil
}

func (t *ShardingTx) VerifySignature(pubKey crypto.PubKeyEd25519, signature crypto.SignatureEd25519) bool {
	// tBytes, err := json.Marshal(t)
	// if err != nil {
	// 	return false
	// }
	// pubKeyBytes := [32]byte(pubKey)
	// sig := [64]byte(signature)
	// return ed25519.Verify(&pubKeyBytes, tBytes, &sig)
	return true
}

var (
	lastBlockKey = []byte("lastblock")

	ErrUnknownTx   = fmt.Errorf("unknown tx")
	ErrShardExists = fmt.Errorf("shard exists")
	ErrFailToStop  = fmt.Errorf("stop shard failed")
)

func NewShardingApp(logger *zap.Logger, conf config.Config) *ShardingApp {
	datadir := conf.GetString("db_dir")
	app := ShardingApp{
		config: conf,
		logger: logger,

		Txs:             make([][]byte, 0),
		RunningChainIDs: make(map[string]string),
		Shards:          make(map[string]*ShardNode),
	}

	var err error
	if app.chainDb, err = db.NewGoLevelDB("chaindata", datadir); err != nil {
		cmn.PanicCrisis(err)
	}

	app.engineHooks = types.Hooks{
		OnExecute: types.NewHook(app.OnExecute),
		OnCommit:  types.NewHook(app.OnCommit),
	}

	return &app
}

func (app *ShardingApp) Lock() {
	app.mtx.Lock()
}

func (app *ShardingApp) Unlock() {
	app.mtx.Unlock()
}

func (app *ShardingApp) setNode(n *Node) {
	app.node = n
}

// Stop stops all still running shards
func (app *ShardingApp) Stop() {
	for i := range app.Shards {
		app.Shards[i].Stop()
	}
}

// Start will restore all shards according to tx history
func (app *ShardingApp) Start() {
	lastBlock := app.LoadLastBlock()
	if lastBlock.Txs != nil && len(lastBlock.Txs) > 0 {
		app.Txs = lastBlock.Txs
		for _, tx := range lastBlock.Txs {
			app.ExecuteTx(nil, tx, 0)
		}
	}
}

func (app *ShardingApp) GetAngineHooks() types.Hooks {
	return app.engineHooks
}

func (app *ShardingApp) CompatibleWithAngine() {}

// ExecuteTx execute tx one by one in the loop, without lock, so should always be called between Lock() and Unlock() on the *stateDup
func (app *ShardingApp) ExecuteTx(blockHash []byte, bs []byte, txIndex int) (validtx []byte, err error) {
	if !app.IsShardingTx(bs) {
		return nil, ErrUnknownTx
	}

	txBytes := types.UnwrapTx(bs)
	tx := ShardingTx{}
	if err := json.Unmarshal(txBytes, &tx); err != nil {
		app.logger.Info("Unmarshal tx failed", zap.Binary("tx", txBytes), zap.String("error", err.Error()))
		return nil, err
	}

	sig := tx.Signature
	tx.Signature = [64]byte{}
	pubkey, ok := app.node.PrivValidator().PubKey.(crypto.PubKeyEd25519)
	if !ok {
		return nil, fmt.Errorf("node's pubkey must be crypto.PubKeyEd25519")
	}

	if !tx.VerifySignature(pubkey, sig) {
		app.logger.Debug("this tx is not for me", zap.Binary("tx", bs))
		return bs, nil
	}

	_, srcExist := managedState.accounts[tx.SourceAddress]
	_, destExist := managedState.accounts[tx.DestAddress]
	if srcExist && destExist {
		managedState.accounts[tx.SourceAddress] += tx.Amount
		managedState.accounts[tx.DestAddress] -= tx.Amount
	} else {
		return nil, ErrUnknownTx
	}

	switch tx.Act {
	case ShardJoin:
		app.mtx.Lock()
		if _, ok := app.RunningChainIDs[tx.ChainID]; ok {
			app.mtx.Unlock()
			return nil, ErrShardExists
		}
		app.mtx.Unlock()

		if app.joinShard(&tx); err != nil {
			return nil, err
		}
		return bs, nil
	case ShardLeave:
		app.mtx.Lock()
		if _, ok := app.RunningChainIDs[tx.ChainID]; !ok {
			app.mtx.Unlock()
			return nil, ErrInvalidChainID
		}
		app.mtx.Unlock()

		if err := app.leaveShard(&tx); err != nil {
			return nil, err
		}
		return bs, nil

	default:
		return nil, fmt.Errorf("unimplemented")
	}
}

// OnExecute would not care about Block.ExTxs
func (app *ShardingApp) OnExecute(height, round int, block *types.Block) (interface{}, error) {
	var (
		res types.ExecuteResult
		err error

		blockHash = block.Hash()
	)

	for i := range block.Txs {
		vtx, err := app.ExecuteTx(blockHash, block.Txs[i], i)
		if err == nil {
			res.ValidTxs = append(res.ValidTxs, vtx)
		} else {
			if err == ErrUnknownTx {
				// maybe we could do something with another app or so
			} else {
				res.InvalidTxs = append(res.InvalidTxs, types.ExecuteInvalidTx{Bytes: block.Txs[i], Error: err})
			}
		}
	}

	app.Txs = append(app.Txs, res.ValidTxs...)

	return res, err
}

// OnCommit run in a sync way, we don't need to lock stateDupMtx, but stateMtx is still needed
func (app *ShardingApp) OnCommit(height, round int, block *types.Block) (interface{}, error) {
	lastBlock := LastBlockInfo{Height: uint64(height), Txs: app.Txs, Hash: merkle.SimpleHashFromHashes(app.Txs)}
	app.SaveLastBlock(lastBlock)
	return types.CommitResult{AppHash: lastBlock.Hash}, nil
}

func (app *ShardingApp) LoadLastBlock() (lastBlock LastBlockInfo) {
	buf := app.chainDb.Get(lastBlockKey)
	if len(buf) != 0 {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&lastBlock, r, 0, n, err)
		if *err != nil {

		}
	}
	return lastBlock
}

func (app *ShardingApp) SaveLastBlock(lastBlock LastBlockInfo) {
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(lastBlock, buf, n, err)
	if *err != nil {
		cmn.PanicCrisis(*err)
	}
	app.chainDb.SetSync(lastBlockKey, buf.Bytes())
}

func (app *ShardingApp) CheckTx(bs []byte) error {
	return nil

	// txBytes := types.UnwrapTx(bs)
	// tx := ShardingTx{}
	// if err := json.Unmarshal(txBytes, &tx); err != nil {
	// 	app.logger.Info("Unmarshal tx failed", zap.Binary("tx", txBytes), zap.String("error", err.Error()))
	// 	return err
	// }

	// sig := tx.Signature
	// tx.Signature = [64]byte{}
	// pubkey, ok := app.node.PrivValidator().PubKey.(crypto.PubKeyEd25519)
	// if !ok {
	// 	return fmt.Errorf("my key is not crypto.PubKeyEd25519")
	// }
	// if !tx.VerifySignature(pubkey, sig) {
	// 	return fmt.Errorf("wrong tx for wrong node: %X", pubkey)
	// }

	// return nil
}

func (app *ShardingApp) Info() (resInfo types.ResultInfo) {
	lb := app.LoadLastBlock()
	resInfo.LastBlockAppHash = lb.Hash
	resInfo.LastBlockHeight = lb.Height
	resInfo.Version = "alpha 0.1"
	resInfo.Data = "default app with sharding-controls"
	return
}

func (app *ShardingApp) Query(query []byte) types.Result {
	if len(query) == 0 {
		return types.NewResultOK([]byte{}, "Empty query")
	}
	var res types.Result
	action := query[0]
	// _ = query[1:]
	switch action {
	default:
		res = types.NewError(types.CodeType_BaseInvalidInput, "unimplemented query")
	}

	return res
}

func (app *ShardingApp) IsShardingTx(tx []byte) bool {
	return string(tx[:4]) == ShardTag
}

func (app *ShardingApp) joinShard(tx *ShardingTx) error {
	if !AppExists(tx.App) {
		return fmt.Errorf("no such app: %s", tx.App)
	}
	shardConf, err := acfg.LoadDefaultShardConfig(app.config.GetString("datadir"), tx.ChainID, tx.Config)
	if err != nil {
		return err
	}
	privKey := app.node.privValidator.PrivKey
	pubKey := app.node.privValidator.PubKey
	addr := app.node.privValidator.Address
	shardPrivVal := &types.PrivValidator{
		Address: addr,
		PrivKey: privKey,
		PubKey:  pubKey,
		Signer:  types.NewDefaultSigner(privKey),
	}
	shardPrivVal.SetFile(shardConf.GetString("priv_validator_file"))
	shardPrivVal.Save()
	if err := tx.Genesis.SaveAs(shardConf.GetString("genesis_file")); err != nil {
		app.logger.Error("fail to save shard genesis", zap.String("chainid", tx.ChainID), zap.String("path", shardConf.GetString("genesis_file")))
	}
	shard := NewShardNode(app.logger, tx.App, shardConf)
	app.SetShard(tx.ChainID, tx.App, shard)
	shard.Start()
	return nil
}

func (app *ShardingApp) leaveShard(tx *ShardingTx) error {
	shard, _ := app.GetShard(tx.ChainID)
	if ok := shard.Stop(); !ok {
		return ErrFailToStop
	}
	if err := app.RemoveShard(tx.ChainID); err != nil {
		return err
	}

	return nil
}

func (app *ShardingApp) SetShard(chainID, appname string, s *ShardNode) {
	app.mtx.Lock()
	app.Shards[chainID] = s
	app.RunningChainIDs[chainID] = appname
	app.mtx.Unlock()
}

func (app *ShardingApp) GetShard(chainID string) (*ShardNode, error) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	if s, ok := app.Shards[chainID]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("no such shard: %s", chainID)
}

func (app *ShardingApp) RemoveShard(chainID string) error {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	if app.Shards[chainID].IsRunning() {
		return fmt.Errorf("shard node is still running, error cowordly")
	}
	delete(app.Shards, chainID)
	delete(app.RunningChainIDs, chainID)
	return nil
}
