package history

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type SIDResponse struct {
	SID string `json:"sid"`
}

type Point struct {
	Ts  float64 `json:"ts"`
	Val float64 `json:"val"`
}

type HistoryRequest struct {
	Ack       bool    `json:"ack"`
	AddID     bool    `json:"addID"`
	Aggregate string  `json:"aggregate"`
	Count     string  `json:"count"`
	End       float64 `json:"end"`
	From      bool    `json:"from"`
	Instance  string  `json:"instance"`
	Q         bool    `json:"q"`
	Start     float64 `json:"start"`
}

type historyResult struct {
	Points []Point
	Err    error
}

const reconnectInterval = time.Second * 5

type HistoryService struct {
	host       string
	instance   string
	debug      bool
	logger     *slog.Logger
	connection *websocket.Conn
	writeMu    sync.Mutex
	pendingMu  sync.Mutex
	pending    map[int]chan historyResult
	nextID     atomic.Int64
}

func NewHistoryService(logger *slog.Logger, host string, instance string, debug bool) *HistoryService {
	return &HistoryService{
		host:     host,
		instance: instance,
		debug:    debug,
		logger:   logger,
		pending:  make(map[int]chan historyResult),
	}
}

func (historyService *HistoryService) Connect() {
	for {
		shouldReconnect := make(chan struct{}, 1)
		sid, err := historyService.getSID()
		if err != nil {
			historyService.logger.Error("could not get sid", "error", err)
			return
		}
		connection, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s?ws=true&EIO=3&transport=websocket&sid=%s", historyService.host, sid), nil)
		if err != nil {
			historyService.logger.Error("could not connect to socket", "error", err)
			return
		}
		historyService.connection = connection
		go historyService.listen(shouldReconnect)
		go historyService.ping()
		historyService.logger.Info("connected", "host", historyService.host)
		_ = historyService.sendMessage("2probe")
		_ = historyService.sendMessage("5")
		_ = historyService.sendMessage(fmt.Sprintf("42%d[\"authenticate\"]", int(historyService.nextID.Add(1))))
		<-shouldReconnect
		historyService.logger.Info("reconnecting", "host", historyService.host)
		time.Sleep(reconnectInterval)
	}
}

func (historyService *HistoryService) ping() {
	for {
		time.Sleep(time.Minute)
		_ = historyService.sendMessage("2")
	}
}
func (historyService *HistoryService) GetHistory(id string, count string, start float64, end float64) ([]Point, error) {
	reqestID := int(historyService.nextID.Add(1))
	replyChannel := make(chan historyResult, 1)
	historyService.pendingMu.Lock()
	historyService.pending[reqestID] = replyChannel
	historyService.pendingMu.Unlock()
	defer func() {
		historyService.pendingMu.Lock()
		delete(historyService.pending, reqestID)
		historyService.pendingMu.Unlock()
	}()

	historyRequestPayload := &HistoryRequest{
		Ack:       false,
		AddID:     false,
		Aggregate: "minmax",
		Count:     count,
		End:       end,
		From:      false,
		Instance:  historyService.instance,
		Q:         false,
		Start:     start,
	}
	historyRequest := [3]any{"getHistory", id, historyRequestPayload}
	buffer := &bytes.Buffer{}
	if err := json.NewEncoder(buffer).Encode(historyRequest); err != nil {
		return nil, fmt.Errorf("error enconding history request: %w", err)
	}
	message := fmt.Sprintf("42%d%s", reqestID, strings.TrimSpace(buffer.String()))
	if err := historyService.sendMessage(message); err != nil {
		return nil, err
	}
	select {
	case res := <-replyChannel:
		return res.Points, res.Err
	case <-time.After(5 * time.Second):
		return nil, errors.New("response timeout")
	}
}

func (historyService *HistoryService) listen(shouldReconnect chan<- struct{}) {
	defer func() {
		historyService.failAllPendingRequests(errors.New("connection closed"))
		shouldReconnect <- struct{}{}
	}()

	for {
		_, message, err := historyService.connection.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		trimmedMessage := strings.TrimSpace(string(message))
		if historyService.debug {
			historyService.logger.Info("received message", "message", trimmedMessage)
		}
		if trimmedMessage == "3" || trimmedMessage == "3probe" || trimmedMessage == "431[true,null]" {
			continue
		}
		bracketIndex := strings.IndexByte(trimmedMessage, '[')
		if bracketIndex == -1 {
			historyService.logger.Warn("unknown message", "message", trimmedMessage)
			continue
		}
		prefix := trimmedMessage[:bracketIndex]
		jsonPart := trimmedMessage[bracketIndex:]
		if !strings.HasPrefix(prefix, "43") {
			continue
		}
		ackIDStr := strings.TrimPrefix(prefix, "43")
		ackID, err := strconv.Atoi(ackIDStr)
		if err != nil {
			historyService.logger.Warn("invalid ack id", "message", trimmedMessage)
			continue
		}

		historyService.pendingMu.Lock()
		replyChannel := historyService.pending[ackID]
		historyService.pendingMu.Unlock()
		if replyChannel == nil {
			continue
		}

		raw := []json.RawMessage{}
		if err := json.NewDecoder(strings.NewReader(jsonPart)).Decode(&raw); err != nil {
			select {
			case replyChannel <- historyResult{Err: fmt.Errorf("error parsing message: %w", err)}:
			default:
			}
			continue
		}
		if len(raw) < 2 {
			select {
			case replyChannel <- historyResult{Err: fmt.Errorf("unexpected ack payload: %s", jsonPart)}:
			default:
			}
			continue
		}

		var points []Point
		if err := json.NewDecoder(bytes.NewReader(raw[1])).Decode(&points); err != nil {
			select {
			case replyChannel <- historyResult{Err: fmt.Errorf("error parsing points: %w", err)}:
			default:
			}
			continue
		}

		select {
		case replyChannel <- historyResult{Points: points}:
		default:
		}
	}
}

func (s *HistoryService) failAllPendingRequests(err error) {
	s.pendingMu.Lock()
	pending := s.pending
	s.pending = make(map[int]chan historyResult)
	s.pendingMu.Unlock()

	for _, ch := range pending {
		select {
		case ch <- historyResult{Err: err}:
		default:
		}
	}
}

func (historyService *HistoryService) sendMessage(message string) error {
	historyService.writeMu.Lock()
	defer historyService.writeMu.Unlock()
	if historyService.connection == nil {
		return errors.New("connection is nil")
	}
	if historyService.debug {
		historyService.logger.Info("send message", "message", message)
	}
	err := historyService.connection.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		historyService.logger.Error("write failed", "error", err, "message", message)
		return err
	}
	return nil
}

func (historyService *HistoryService) getSID() (string, error) {
	response, err := http.Get(fmt.Sprintf("http://%s?ws=true&EIO=3&transport=polling", historyService.host))
	if err != nil {
		return "", fmt.Errorf("error creating sid url: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			historyService.logger.Error("close error", "error", err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading body of sid request: %w", err)
	}
	payload := string(body)
	start := strings.Index(payload, "{")
	end := strings.Index(payload, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("error identifying sid response")
	}
	sIDResponse := &SIDResponse{}
	if err := json.NewDecoder(strings.NewReader(payload[start : end+1])).Decode(sIDResponse); err != nil {
		return "", fmt.Errorf("error sid response json decode: %w", err)
	}
	if sIDResponse.SID == "" {
		return "", fmt.Errorf("sid not found")
	}
	return sIDResponse.SID, nil
}
