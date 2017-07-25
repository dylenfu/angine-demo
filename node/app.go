package node

import (
	"bytes"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"gitlab.zhonganonline.com/ann/angine/types"
	cmn "gitlab.zhonganonline.com/ann/ann-module/lib/go-common"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-crypto"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-db"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-wire"
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
	engineHooks types.Hooks
	logger      *zap.Logger
	mtx         sync.Mutex
}

var (
	lastBlockKey   = []byte("lastblock")
	ErrUnknownTx   = fmt.Errorf("unknown tx")
	ErrShardExists = fmt.Errorf("shard exists")
	ErrFailToStop  = fmt.Errorf("stop shard failed")
)

func NewShardingApp(logger *zap.Logger, conf config.Config) *ShardingApp {
	datadir := conf.GetString("db_dir")
	app := ShardingApp{
		config: conf,
		logger: logger,
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

// Start will restore all shards according to tx history
func (app *ShardingApp) Start() {
	lastBlock := app.LoadLastBlock()
	if lastBlock.Txs != nil && len(lastBlock.Txs) > 0 {
		for _, tx := range lastBlock.Txs {
			app.ExecuteTx(nil, tx, 0)
		}
	}
}

// Stop stops all still running shards
func (app *ShardingApp) Stop() {

}

func (app *ShardingApp) GetAngineHooks() types.Hooks {
	return app.engineHooks
}

func (app *ShardingApp) CompatibleWithAngine() {}

func (app *ShardingApp) CheckTx([]byte) error {
	return nil
}

// ExecuteTx execute tx one by one in the loop, without lock, so should always be called between Lock() and Unlock() on the *stateDup
func (app *ShardingApp) ExecuteTx(blockHash []byte, bs []byte, txIndex int) error {
	panic("execute tx not implement yet")
}

// OnExecute would not care about Block.ExTxs
func (app *ShardingApp) OnExecute(height, round int, block *types.Block) (interface{}, error) {
	var (
		res types.ExecuteResult
		err error

		blockHash = block.Hash()
	)

	for i := range block.Txs {
		err := app.ExecuteTx(blockHash, block.Txs[i], i)
		if err == nil {
			//res.ValidTxs = append(res.ValidTxs, vtx)
		}
	}

	return res, err
}

// OnCommit run in a sync way, we don't need to lock stateDupMtx, but stateMtx is still needed
func (app *ShardingApp) OnCommit(height, round int, block *types.Block) (interface{}, error) {
	lastBlock := LastBlockInfo{
		Height: uint64(height),
		Txs:    nil, //app.Txs,
		Hash:   nil, //merkle.SimpleHashFromHashes(app.Txs)
	}

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
