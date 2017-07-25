package main

import (
	"angine-demo/node"
	"path"

	"flag"

	acfg "gitlab.zhonganonline.com/ann/angine/config"
	"gitlab.zhonganonline.com/ann/ann-module/lib/go-config"
	"gitlab.zhonganonline.com/ann/civilwar/src/chain/log"
)

// NodeNum for angine prevote
const (
	root    = "/home/vagrant/gohome/src/angine-demo/"
	logpath = root + "log/"
)

// DataDirFlag set seed direction
var (
	DataDirFlag  = flag.String("datadir", "seed0", "set seed data dir")
	InitSeedFlag = flag.Bool("init", false, "new config files")

	annConf *config.MapConfig
)

func main() {
	flag.Parse()
	annConf = acfg.GetConfig(root + *DataDirFlag)
	env := annConf.GetString("environment")
	logger := log.Initialize(env, path.Join(logpath, "node.output.log"), path.Join(logpath, "node.err.log"))

	if *InitSeedFlag == true {
		node.Initfiles(annConf)
	} else {
		seed := node.New(logger, annConf)
		seed.Run()
	}
}
