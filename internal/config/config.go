package config

import "path"

type Config struct {
	DataDir    string
	ListenAddr string
}

func Load(dataDir string) (Config, error) {
	if dataDir == "" {
		dataDir = "/data"
	}

	return Config{
		DataDir:    path.Clean(dataDir),
		ListenAddr: ":8080",
	}, nil
}
