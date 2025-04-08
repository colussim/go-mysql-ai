package config

import (
	"encoding/json"
	"os"

	"github.com/sirupsen/logrus"
)

type Config struct {
	MySQL struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Server   string `json:"server"`
		Port     string `json:"port"`
		TypeAuth string `json:"type_auth"`
	} `json:"mysql"`
	Pathologie struct {
		File string `json:"file"`
	} `json:"pathologie"`
	Models struct {
		Embedding struct {
			Name string `json:"name"`
		} `json:"embedding"`
		Generation struct {
			Name   string `json:"name"`
			Prompt string `json:"prompt"`
		} `json:"generation"`
	} `json:"models"`
	Chatbotport struct {
		Port int `json:"port"`
	} `json:"chatbotport"`
}

type PathologyDetail struct {
	Description string   `json:"description"`
	Symptoms    []string `json:"symptoms"`
	Treatments  []string `json:"treatments"`
}

type Pathology struct {
	Pathologies map[string]PathologyDetail `json:"pathologies"`
}

var Log = logrus.New()

func InitLogger() {
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func LoadPathologies(filename string) (*Pathology, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var pathologies Pathology
	if err := json.Unmarshal(file, &pathologies); err != nil {
		return nil, err
	}
	return &pathologies, nil
}
