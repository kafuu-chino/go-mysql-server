package index

import (
	"io"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

// Config represents index configuration
type Config struct {
	DB          string
	Table       string
	ID          string
	Expressions []string
	Drivers     map[string]map[string]string
}

// NewConfig creates a new Config instance for given driver's configuration
func NewConfig(db, table, id string,
	expressions []string,
	driverID string,
	driverConfig map[string]string) *Config {

	cfg := &Config{
		DB:          db,
		Table:       table,
		ID:          id,
		Expressions: expressions,
		Drivers:     make(map[string]map[string]string),
	}
	cfg.Drivers[driverID] = driverConfig

	return cfg
}

// Driver returns an configuration for the particular driverID.
func (cfg *Config) Driver(driverID string) map[string]string {
	return cfg.Drivers[driverID]
}

// WriteConfig writes the configuration to the passed writer (w).
func WriteConfig(w io.Writer, cfg *Config) error {
	data, err := yaml.Marshal(cfg)

	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// WriteConfigFile writes the configuration to file.
func WriteConfigFile(file string, cfg *Config) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	return WriteConfig(f, cfg)
}

// ReadConfig reads an configuration from the passed reader (r).
func ReadConfig(r io.Reader) (*Config, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}

// ReadConfigFile reads an configuration from  file.
func ReadConfigFile(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ReadConfig(f)
}

// CreateProcessingFile creates a file  saying whether the index is being created.
func CreateProcessingFile(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	// we don't care about errors closing here
	_ = f.Close()
	return nil
}

// RemoveProcessingFile removes the file that says whether the index is still being created.
func RemoveProcessingFile(file string) error {
	return os.Remove(file)
}

// ExistsProcessingFile returns whether the processing file exists.
func ExistsProcessingFile(file string) (bool, error) {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
