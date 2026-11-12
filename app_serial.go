package main

import (
	"encoding/base64"
	"fmt"
	"time"

	serialbackend "conch/backend/serial"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SerialConnectRequest holds serial port parameters from the frontend.
type SerialConnectRequest struct {
	Port     string `json:"port"`
	BaudRate int    `json:"baud_rate"`
	DataBits int    `json:"data_bits"`
	Parity   string `json:"parity"`    // none / odd / even
	StopBits string `json:"stop_bits"` // "1" / "1.5" / "2"
}

// SerialListPorts returns available serial ports with friendly descriptions
// (e.g. "Silicon Labs CP210x USB to UART Bridge").
func (a *App) SerialListPorts() ([]serialbackend.PortInfo, error) {
	return serialbackend.ListPorts()
}

// SerialConnect opens a serial port session and returns a connID.
// Output streams to "terminal:output:{connID}" (base64) exactly like SSH
// sessions, so the frontend terminal needs no special-casing; on port loss a
// "terminal:closed:{connID}" event is emitted.
func (a *App) SerialConnect(req SerialConnectRequest) (string, error) {
	connID := uuid.New().String()

	onOutput := func(data []byte) {
		enc := base64.StdEncoding.EncodeToString(data)
		entry := a.getConn(connID)
		if entry != nil {
			entry.outputMu.Lock()
			if !entry.ready {
				entry.outputBuf = append(entry.outputBuf, enc)
				entry.outputMu.Unlock()
				return
			}
			entry.outputMu.Unlock()
		}
		runtime.EventsEmit(a.ctx, "terminal:output:"+connID, enc)
	}
	onClose := func() {
		if a.getConn(connID) == nil {
			return // already disconnected manually
		}
		runtime.EventsEmit(a.ctx, "terminal:closed:"+connID)
		a.removeConn(connID)
	}

	sess, err := serialbackend.Open(serialbackend.Options{
		Port:     req.Port,
		BaudRate: req.BaudRate,
		DataBits: req.DataBits,
		Parity:   req.Parity,
		StopBits: req.StopBits,
	}, onOutput, onClose)
	if err != nil {
		return "", err
	}

	a.connMu.Lock()
	a.connections[connID] = &connEntry{serial: sess}
	a.connMu.Unlock()

	// 串口设备通常不会主动输出，屏幕全黑易被误判为卡死：
	// 先打印一条本地状态行，再自动发一个回车唤醒对端控制台提示符
	onOutput([]byte(fmt.Sprintf("\x1b[90m─── 已连接 %s @ %d ───\x1b[0m\r\n", req.Port, req.BaudRate)))
	go func() {
		time.Sleep(300 * time.Millisecond)
		sess.Write([]byte("\r"))
	}()

	return connID, nil
}
