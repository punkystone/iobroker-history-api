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
	"strings"
	"time"
	"unicode"

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

type HistoryService struct {
	host            string
	instance        string
	logger          *slog.Logger
	connection      *websocket.Conn
	sendCounter     int
	responseChannel chan []Point
}

func NewHistoryService(logger *slog.Logger, host string, instance string) *HistoryService {
	return &HistoryService{
		host:            host,
		instance:        instance,
		logger:          logger,
		sendCounter:     1,
		responseChannel: make(chan []Point),
	}
}

func (historyService *HistoryService) Connect() {
	sid, err := historyService.getSID()
	if err != nil {
		historyService.logger.Error("could not get sid", "error", err)
		return
	}
	historyService.logger.Info(sid)
	connection, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s?ws=true&EIO=3&transport=websocket&sid=%s", historyService.host, sid), nil)
	if err != nil {
		historyService.logger.Error("could not connect to socket", "error", err)
		return
	}
	historyService.connection = connection
	historyService.sendCounter = 1
	historyService.logger.Info("connected")
	go historyService.listen()
	go historyService.ping()
	historyService.sendMessage("2probe", false)
	historyService.sendMessage("5", false)
	historyService.sendMessage(fmt.Sprintf("42%d[\"authenticate\"]", historyService.sendCounter), true)
	time.Sleep(time.Second * 5)
	/*points, err := historyService.getHistory("mqtt.0.tgn.frient.ElecMeter_1.power", "300", 1776095119857.426, 1776103744737.426)
	if err != nil {
		fmt.Println("timeout")
	}
	fmt.Println(len(points))
	points, err = historyService.getHistory("mqtt.0.tgn.frient.ElecMeter_1.power", "300", 1776095119857.426, 1776103744737.426)
	if err != nil {
		fmt.Println("timeout")
	}
	fmt.Println(len(points))*/
}

func (historyService *HistoryService) ping() {
	for {
		time.Sleep(time.Minute)
		historyService.sendMessage("2", false)
	}
}
func (historyService *HistoryService) getHistory(id string, count string, start float64, end float64) ([]Point, error) {
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
	historyService.sendMessage(fmt.Sprintf("42%d%s", historyService.sendCounter, buffer.String()), true)
	select {
	case v := <-historyService.responseChannel:
		return v, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("response timeout")
	}
}

func (historyService *HistoryService) listen() {
	for {
		_, message, err := historyService.connection.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		trimmedMessage := strings.TrimSpace(string(message))
		if trimmedMessage == "3" || trimmedMessage == "3probe" {
			continue
		}
		bracketIndex := strings.IndexByte(trimmedMessage, '[')
		if bracketIndex == -1 {
			historyService.logger.Warn("unknown message", "message", trimmedMessage)
			continue
		}
		for _, r := range trimmedMessage[:bracketIndex] {
			if !unicode.IsDigit(r) {
				historyService.logger.Warn("unknown message", "message", trimmedMessage)
				continue
			}
		}
		jsonPart := trimmedMessage[bracketIndex:]
		raw := []json.RawMessage{}
		if err := json.NewDecoder(strings.NewReader(jsonPart)).Decode(&raw); err != nil {
			historyService.logger.Warn("error parsing message", "error", err, "message", trimmedMessage)
			continue
		}
		if len(raw) < 3 {
			historyService.logger.Warn("unknown message", "message", trimmedMessage)
			continue
		}
		points := []Point{}
		if err := json.NewDecoder(bytes.NewReader(raw[1])).Decode(&points); err != nil {
			historyService.logger.Warn("error parsing message", "error", err, "message", trimmedMessage)
			continue
		}
		historyService.responseChannel <- points
	}
	//reconnect
}

func (historyService *HistoryService) sendMessage(message string, incrementSendCounter bool) {
	if historyService.connection == nil {
		return
	}
	historyService.logger.Info(message)
	err := historyService.connection.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		historyService.logger.Error("write failed", "error", err, "message", message)
		return
	}
	if incrementSendCounter {
		historyService.sendCounter += 1
	}
}

func (historyService *HistoryService) getSID() (string, error) {
	response, err := http.Get(fmt.Sprintf("http://%s?ws=true&EIO=3&transport=polling", historyService.host))
	if err != nil {
		return "", fmt.Errorf("error creating sid url: %w", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading body of sid request: %w", err)
	}
	payload := string(body)
	historyService.logger.Info(payload)
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
