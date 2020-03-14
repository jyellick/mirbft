package main

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/IBM/mirbft/example/network"
	"github.com/perlin-network/noise"
	"gopkg.in/yaml.v2"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var BasePort = 5000

func main() {
	count, err := strconv.Atoi(os.Args[1])
	check(err)

	var peers []network.Peer
	var privkeys []string

	for i := 0; i < count; i++ {
		pubkey, privkey, err := noise.GenerateKeys(nil)
		check(err)

		privkeys = append(privkeys, privkey.String())

		peer := network.Peer{
			ID:        uint64(i),
			Address:   "127.0.0.1:" + strconv.Itoa(BasePort+i),
			PublicKey: pubkey.String(),
		}
		peers = append(peers, peer)
	}

	for i := 0; i < count; i++ {
		config := network.Config{
			ID:            uint64(i),
			ListenAddress: peers[i].Address,
			PrivateKey:    privkeys[i],
			Peers:         peers,
		}
		out, err := yaml.Marshal(config)
		check(err)

		err = ioutil.WriteFile("config"+strconv.Itoa(i)+".yaml", out, 0644)
		check(err)
	}
}
