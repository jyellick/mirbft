package network

import (
	"encoding/hex"
	"io/ioutil"
	"net"
	"strconv"

	"github.com/perlin-network/noise"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	ID            uint64 `yaml:"id"`
	ListenAddress string `yaml:"listen_address"`
	PrivateKey    string `yaml:"private_key"`
	Peers         []Peer `yaml:"peers"`
}

type Peer struct {
	ID        uint64 `yaml:"id"`
	Address   string `yaml:"address"`
	PublicKey string `yaml:"public_key"`
}

func LoadConfig(f string) (*Config, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, errors.Errorf("fail to read config file: %s", err)
	}

	config := &Config{}
	if err = yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

func NewID(config *Config) (*noise.ID, error) {
	key, err := hex.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, err
	}

	var privkey noise.PrivateKey
	copy(privkey[:], key)

	hostStr, portStr, err := net.SplitHostPort(config.ListenAddress)
	if err != nil {
		return nil, err
	}

	host := net.ParseIP(hostStr)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	return &noise.ID{
		ID:      privkey.Public(),
		Host:    host,
		Port:    uint16(port),
		Address: config.ListenAddress,
	}, nil
}
