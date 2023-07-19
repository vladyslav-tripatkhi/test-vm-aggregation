package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type labels map[string]string

func (l labels) String() string {
	tmpSlice := make([]string, 0, len(l))

	for k, v := range l {
		tmpSlice = append(tmpSlice, fmt.Sprintf(`%s="%s"`, k, v))
	}

	sort.Strings(tmpSlice)
	return strings.Join(tmpSlice, ",")
}

type metric struct {
	Type     string        `yaml:"type"`
	Name     string        `yaml:"name"`
	Labels   labels        `yaml:"labels"`
	Value    int           `yaml:"value"`
	Interval time.Duration `yaml:"interval"`
}

type config struct {
	VmImportUrl   string        `yaml:"vm_import_url"`
	Host          string        `yaml:"host"`
	Port          int           `yaml:"port"`
	DefaultLabels labels        `yaml:"default_labels"`
	Metrics       []*metric     `yaml:"metrics"`
	PushInterval  time.Duration `yaml:"push_interval"`

	set *metrics.Set
}

func newConfig(fileName string) (*config, error) {
	if fileName == "" {
		fileName = "./default_config.yml"
		log.Infof("Config file not provided, falling back to %s", fileName)
	}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var c *config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	if c.VmImportUrl == "" {
		c.VmImportUrl = "http://localhost:8428/api/v1/import/prometheus"
	}

	if c.Port == 0 {
		c.Port = 8080
	}

	if c.PushInterval == 0 {
		c.PushInterval = 10 * time.Second
	}

	c.set = metrics.NewSet()
	c.set.InitPush(c.VmImportUrl, c.PushInterval, c.DefaultLabels.String())

	return c, nil
}

func (c *config) GetListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *config) GetMetrics(w http.ResponseWriter) {
	c.set.WritePrometheus(w)
}

func (c *config) Start() {
	for _, counter := range c.Metrics {
		go counter.Send(c.set)
	}
}

func (m *metric) Send(s *metrics.Set) {
	fullName := fmt.Sprintf("%s{%s}", m.Name, m.Labels)
	log.Infof("sending metric %s with value %d every %v", fullName, m.Value, m.Interval)

	for {
		switch m.Type {
		case "histogram":
			h := s.GetOrCreateHistogram(fullName)
			h.Update(float64(m.Value))
		case "summary":
			s := s.GetOrCreateSummary(fullName)
			s.Update(float64(m.Value))
		default:
			c := s.GetOrCreateCounter(fullName)
			c.Add(m.Value)
		}
		time.Sleep(m.Interval)
	}
}

func main() {
	log.SetLevel(log.InfoLevel)

	log.Info("Starting program execution!")
	c, err := newConfig(os.Getenv("CONFIG_FILE_NAME"))
	if err != nil {
		log.Fatalf("Cannot parse config: %s", err)
	}

	c.Start()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		c.GetMetrics(w)
	})

	if err := http.ListenAndServe(c.GetListenAddr(), nil); err != nil {
		log.Fatal(err)
	}
}
