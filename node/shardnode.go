package node

import (
	"fmt"
	"sync/atomic"

	"go.uber.org/zap"

	"gitlab.zhonganonline.com/ann/angine"
	"gitlab.zhonganonline.com/ann/angine/types"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	client "gitlab.zhonganonline.com/ann/ann-module/lib/go-rpc/client"
)

const (
	RPCCollectSpecialVotes uint8 = iota
)

type ShardNode struct {
	running int64

	logger *zap.Logger

	Angine      *angine.Angine
	AngineTune  *angine.AngineTunes
	Application types.Application
	GenesisDoc  *types.GenesisDoc
}

func NewShardNode(logger *zap.Logger, appName string, conf config.Config) *ShardNode {
	if _, ok := Apps[appName]; !ok {
		return nil
	}
	app := Apps[appName](conf)
	tune := &angine.AngineTunes{Conf: conf.(*config.MapConfig)}
	engine := angine.NewAngine(tune)
	engine.ConnectApp(app)
	shard := &ShardNode{
		logger: logger,

		Application: app,
		Angine:      engine,
		AngineTune:  tune,
		GenesisDoc:  engine.Genesis(),
	}

	engine.SetSpecialVoteRPC(shard.GetSpecialVote)

	return shard
}

func (s *ShardNode) Start() error {
	if atomic.CompareAndSwapInt64(&s.running, 0, 1) {
		s.Application.Start()
		return s.Angine.Start()
	}
	return fmt.Errorf("already started")
}

func (s *ShardNode) Stop() bool {
	if atomic.CompareAndSwapInt64(&s.running, 1, 0) {
		s.Application.Stop()
		return s.Angine.Stop()
	}
	return false
}

func (s *ShardNode) IsRunning() bool {
	return atomic.LoadInt64(&s.running) == 1
}

func (s *ShardNode) GetSpecialVote(data []byte, validator *types.Validator) ([]byte, error) {
	clientJSON := client.NewClientJSONRPC(s.logger, validator.RPCAddress) // all shard nodes share the same rpc address of the Node
	tmResult := new(types.RPCResult)
	_, err := clientJSON.Call("vote_special_op", []interface{}{s.GenesisDoc.ChainID, data}, tmResult)
	if err != nil {
		return nil, err
	}
	res := (*tmResult).(*types.ResultRequestSpecialOP)
	if res.Code == types.CodeType_OK {
		return res.Data, nil
	}
	return nil, fmt.Errorf(res.Log)
}
