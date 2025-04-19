package config

import (
	"io/ioutil"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	config     *Config
	configLock sync.RWMutex
)

// Config representa la configuración de la aplicación
type Config struct {
	General struct {
		DataDir string `yaml:"data_dir"`
	} `yaml:"general"`

	API struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	} `yaml:"api"`

	WebSocket struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	} `yaml:"websocket"`

	Database struct {
		Backup struct {
			AutoBackup bool `yaml:"auto_backup"`
			Interval   int  `yaml:"interval"`
			MaxBackups int  `yaml:"max_backups"`
		} `yaml:"backup"`
	} `yaml:"database"`

	Network struct {
		LibP2P struct {
			ListenAddresses []string `yaml:"listen_addresses"`
			BootstrapPeers  []string `yaml:"bootstrap_peers"`
		} `yaml:"libp2p"`

		MDNS struct {
			Enabled     bool   `yaml:"enabled"`
			ServiceName string `yaml:"service_name"`
			Interval    int    `yaml:"interval"`
		} `yaml:"mdns"`

		DHT struct {
			Enabled           bool   `yaml:"enabled"`
			Mode              string `yaml:"mode"`
			BootstrapInterval int    `yaml:"bootstrap_interval"`
		} `yaml:"dht"`
	} `yaml:"network"`

	Auth struct {
		JWT struct {
			Secret     string `yaml:"secret"`
			Expiration int    `yaml:"expiration"`
		} `yaml:"jwt"`

		DefaultAdmin struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"default_admin"`
	} `yaml:"auth"`
}

// LoadConfig carga la configuración desde un archivo YAML
func LoadConfig(filePath string) (*Config, error) {
	configLock.Lock()
	defer configLock.Unlock()

	// Leer el archivo de configuración
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Deserializar el YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Establecer valores predeterminados si no están definidos
	if cfg.General.DataDir == "" {
		cfg.General.DataDir = "./data"
	}

	if cfg.API.Port == 0 {
		cfg.API.Port = 8080
	}

	if cfg.WebSocket.Port == 0 {
		cfg.WebSocket.Port = 8081
	}

	// Guardar la configuración global
	config = &cfg

	return &cfg, nil
}

// GetConfig devuelve la configuración actual o crea una predeterminada
func GetConfig() *Config {
	configLock.RLock()
	if config != nil {
		defer configLock.RUnlock()
		return config
	}
	configLock.RUnlock()

	// Si no hay configuración, crear una predeterminada
	configLock.Lock()
	defer configLock.Unlock()

	if config != nil {
		return config
	}

	// Configuración predeterminada
	config = &Config{}

	// General
	config.General.DataDir = "./data"

	// API
	config.API.Enabled = true
	config.API.Port = 8080

	// WebSocket
	config.WebSocket.Enabled = true
	config.WebSocket.Port = 8081

	// Database
	config.Database.Backup.AutoBackup = true
	config.Database.Backup.Interval = 3600
	config.Database.Backup.MaxBackups = 5

	// Network
	config.Network.LibP2P.ListenAddresses = []string{
		"/ip4/0.0.0.0/tcp/9000",
		"/ip4/0.0.0.0/tcp/9001/ws",
	}
	config.Network.LibP2P.BootstrapPeers = []string{}

	config.Network.MDNS.Enabled = true
	config.Network.MDNS.ServiceName = "dbp2p"
	config.Network.MDNS.Interval = 10

	config.Network.DHT.Enabled = true
	config.Network.DHT.Mode = "client"
	config.Network.DHT.BootstrapInterval = 300

	// Auth
	config.Auth.JWT.Secret = "dbp2p_secret_key"
	config.Auth.JWT.Expiration = 86400

	config.Auth.DefaultAdmin.Username = "admin"
	config.Auth.DefaultAdmin.Password = "admin123"

	// Crear archivo de configuración predeterminado si no existe
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		if data, err := yaml.Marshal(config); err == nil {
			if err := ioutil.WriteFile("config.yaml", data, 0644); err != nil {
				log.Printf("Error al escribir archivo de configuración predeterminado: %v", err)
			}
		}
	}

	return config
}

// SaveConfig guarda la configuración en un archivo
func SaveConfig(cfg *Config, filePath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, data, 0644)
}
