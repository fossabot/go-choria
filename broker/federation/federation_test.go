package federation

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/choria-io/go-choria/mcollective"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

var choria *mcollective.Choria

func init() {
	choria, _ = mcollective.New("testdata/federation.cfg")
}

func newDiscardLogger() (*log.Entry, *bufio.Writer, *bytes.Buffer) {
	var logbuf bytes.Buffer

	logger := log.New().WithFields(log.Fields{"test": "true"})
	logger.Logger.Level = log.DebugLevel
	logtxt := bufio.NewWriter(&logbuf)
	logger.Logger.Out = logtxt

	return logger, logtxt, &logbuf
}

func waitForLogLines(w *bufio.Writer, b *bytes.Buffer) {
	for {
		w.Flush()
		if b.Len() > 0 {
			return
		}
	}

}

type stubConnectionManager struct {
	connection *stubConnection
}

type stubConnection struct {
	Outq        chan [2]string
	Subs        map[string][3]string
	SubChannels map[string]chan *mcollective.ConnectorMessage
	name        string
	mu          *sync.Mutex
}

func (s *stubConnection) Receive() *mcollective.ConnectorMessage {
	return nil
}

func (s *stubConnection) PublishToQueueSub(name string, msg *mcollective.ConnectorMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.SubChannels[name]
	if !ok {
		s.SubChannels[name] = make(chan *mcollective.ConnectorMessage, 1000)
		c = s.SubChannels[name]
	}

	c <- msg
}

func (s *stubConnection) ConnectedServer() string {
	return "nats://stub:4222"
}

func (s *stubConnection) Subscribe(name string, subject string, group string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Subs[name] = [3]string{name, subject, group}

	return nil
}

func (s *stubConnection) Unsubscribe(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Subs[name]; ok {
		delete(s.Subs, name)
	}

	if _, ok := s.SubChannels[name]; ok {
		delete(s.SubChannels, name)
	}

	return nil
}

func (s *stubConnection) ChanQueueSubscribe(name string, subject string, group string, capacity int) (chan *mcollective.ConnectorMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Subs[name] = [3]string{name, subject, group}

	_, ok := s.SubChannels[name]
	if !ok {
		s.SubChannels[name] = make(chan *mcollective.ConnectorMessage, 1000)
	}

	return s.SubChannels[name], nil
}

func (s *stubConnection) PublishRaw(target string, data []byte) error {
	s.Outq <- [2]string{target, string(data)}

	return nil
}

func (s *stubConnection) Connect() error {
	return nil
}

func (s *stubConnection) SetName(name string) {
	s.name = name
}

func (s *stubConnection) SetServers(resolver func() ([]mcollective.Server, error)) {}

func (s *stubConnectionManager) NewConnector(servers func() ([]mcollective.Server, error), name string, logger *log.Entry) (conn mcollective.Connector, err error) {
	if s.connection != nil {
		return s.connection, nil
	}

	conn = &stubConnection{
		Outq:        make(chan [2]string, 64),
		SubChannels: make(map[string]chan *mcollective.ConnectorMessage),
		Subs:        make(map[string][3]string),
		mu:          &sync.Mutex{},
	}

	s.connection = conn.(*stubConnection)

	return
}

func (s *stubConnectionManager) Init() *stubConnectionManager {
	s.connection = &stubConnection{
		Outq:        make(chan [2]string, 64),
		SubChannels: make(map[string]chan *mcollective.ConnectorMessage),
		Subs:        make(map[string][3]string),
		mu:          &sync.Mutex{},
	}

	return s
}
func TestFederation(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Federation")
}

var _ = Describe("Federation Broker", func() {
	It("Should initialize correctly", func() {
		log.SetOutput(ioutil.Discard)

		choria, err := mcollective.New("testdata/federation.cfg")
		Expect(err).ToNot(HaveOccurred())

		fb, err := NewFederationBroker("test_cluster", choria)
		Expect(err).ToNot(HaveOccurred())

		Expect(fb.Stats.Status).To(Equal("unknown"))
		Expect(fb.Stats.CollectiveStats.ConnectedServer).To(Equal("unknown"))
		Expect(fb.Stats.FederationStats.ConnectedServer).To(Equal("unknown"))
		Expect(fb.Stats.StartTime).To(BeTemporally("~", time.Now()))
	})
})