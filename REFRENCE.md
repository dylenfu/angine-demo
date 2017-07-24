civilwar peer start steps:

1.生成build目录及可执行文件
make ann

2.初始化节点
在build目录建立seed1目录，运行
./ann -datadir=seed1 init

3.启动第一个节点
./ann --datadir=seed1 node &
在第二步生成的配置文件在第一个节点启动不需要修改

4.初始化第二个节点后修改config.toml文件(都是服务端口),
environment = "development"
proxy_app = "tcp://127.0.0.1:46658"
moniker = "anonymous"
node_laddr = "tcp://0.0.0.0:46656"
seeds = ""
fast_sync = true
db_backend = "leveldb"
rpc_laddr = "tcp://0.0.0.0:46657"
api_laddr = "tcp://0.0.0.0:46659"
signbyCA = ""

修改genejson文件，添加第一个节点的配置，同时第一个节点seed1/genesis.json
{
  "app_hash": "",
  "chain_id": "annchain-WQxfDn",
  "genesis_time": "0001-01-01T00:00:00.000Z",
  "plugins": "specialop",
  "validators": [
    {
      "amount": 100,
      "is_ca": true,
      "name": "",
      "pub_key": [
      1,
      "DC9F196CCEDC2E51918EDC1F1FF26579B2F619AF22252520CF086919DC977152"
      ],
      "rpc": "tcp://0.0.0.0:36657"
    },
    {
      "amount": 100,
      "is_ca": true,
      "name": "",
      "pub_key": [
      1,
      "3E130C19B205C773A21C5E86E496077E91DE3C479C270B3824FCD450A99AC765"
      ],
      "rpc": "tcp://0.0.0.0:46657"
    },
    {
      "amount": 100,
      "is_ca": true,
      "name": "",
      "pub_key": [
      1,
      "91C809A354013B0DECCCEAE90F5E63855C081D6DE105967ED6E74BF0FEBD40C3"
      ],
      "rpc": "tcp://0.0.0.0:56657"
    }
  ]
}

5.查看log

vagrant@vagrant:~/gohome/src/gitlab.zhonganonline.com/ann/civilwar/build/angine-annchain-WQxfDn$ tail output.log

  Precommits: Vote{0:990BB1BFBA81 1289/00/2(Precommit) 5055902AFC86 /01F8B25D8DEA.../}
  }#8E8874F3AED115A90EB01733A2864ED9B3833C1E
}#F1084472241BBEBE1C3D934DF1C8842B34D6A5E5
2017-07-24T14:56:20.099+0800 INFO state/execution.go:96 Executed block {"height": 1290, "txs": 0, "valid": 0, "invalid": 0, "extended": 0}
2017-07-24T14:56:20.104+0800 DEBUG go-p2p/switch.go:411 Broadcast {"channel": 32, "msg": {"ConsensusMessage":{"Height":1291,"Round":0,"Step":1,"SecondsSinceStartTime":0,"LastCommitRound":0}}}
2017-07-24T14:56:20.104+0800 DEBUG consensus/ticker.go:121 Scheduled timeout {"dur": "990.762414ms", "height": 1291, "round": 0, "step": "RoundStepNewHeight"}
2017-07-24T14:56:20.389+0800 DEBUG blockchain/reactor.go:210 Consensus ticker {"numPending": 300, "total": 300, "outbound": 0, "inbound": 0}
2017-07-24T14:56:20.389+0800 DEBUG blockchain/pool.go:142 Blockpool has no peers
2017-07-24T14:56:20.430+0800 DEBUG blockchain/reactor.go:210 Consensus ticker {"numPending": 300, "total": 300, "outbound": 0, "inbound": 0}
2017-07-24T14:56:20.430+0800 DEBUG blockchain/pool.go:142 Blockpool has no peers

可以看到一直在打新块