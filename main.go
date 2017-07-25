package main

import (
	"angine-demo/node"
	"path"

	"flag"

	acfg "gitlab.zhonganonline.com/ann/angine/config"
	"gitlab.zhonganonline.com/ann/civilwar/src/chain/log"
)

// NodeNum for angine prevote
const (
	root    = "/home/vagrant/gohome/src/angine-demo/"
	logpath = root + "log/"
)

// DataDirFlag set seed direction
var DataDirFlag = flag.String("datadir", "seed0", "set seed data dir")

func main() {
	flag.Parse()
	conf := acfg.GetConfig(root + *DataDirFlag)
	env := conf.GetString("environment")
	logger := log.Initialize(env, path.Join(logpath, "node.output.log"), path.Join(logpath, "node.err.log"))
	node.RunNode(logger, conf)
}
