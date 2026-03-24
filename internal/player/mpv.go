package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

const socketPath = "/tmp/riff-mpv.sock"

type Player struct {
	cmd       *exec.Cmd
	conn      net.Conn
	writeMu   sync.Mutex
	reqID     int
	responses map[int]chan ipcResponse
	respMu    sync.Mutex
	onEnd     chan struct{}
	log       *log.Logger
}

type ipcCommand struct {
	Command   []any `json:"command"`
	RequestID int   `json:"request_id"`
}

type ipcResponse struct {
	Error     string `json:"error"`
	Data      any    `json:"data"`
	RequestID int    `json:"request_id"`
	Event     string `json:"event"`
}

func New(logger *log.Logger) (*Player, error) {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil, fmt.Errorf("mpv not found — install with: brew install mpv")
	}

	os.Remove(socketPath)

	cmd := exec.Command("mpv",
		"--idle",
		"--no-video",
		"--really-quiet",
		fmt.Sprintf("--input-ipc-server=%s", socketPath),
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting mpv: %w", err)
	}

	var conn net.Conn
	for range 50 {
		conn, _ = net.Dial("unix", socketPath)
		if conn != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if conn == nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("timed out waiting for mpv IPC socket")
	}

	p := &Player{
		cmd:       cmd,
		conn:      conn,
		responses: make(map[int]chan ipcResponse),
		onEnd:     make(chan struct{}, 1),
		log:       logger,
	}

	go p.readLoop(bufio.NewReader(conn))

	return p, nil
}

func (p *Player) readLoop(reader *bufio.Reader) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			p.log.Printf("mpv IPC read error: %s", err)
			return
		}

		var resp ipcResponse
		if json.Unmarshal(line, &resp) != nil {
			continue
		}

		if resp.Event == "end-file" {
			select {
			case p.onEnd <- struct{}{}:
			default:
			}
			continue
		}

		if resp.Event != "" {
			continue
		}

		p.respMu.Lock()
		if ch, ok := p.responses[resp.RequestID]; ok {
			ch <- resp
			delete(p.responses, resp.RequestID)
		}
		p.respMu.Unlock()
	}
}

func (p *Player) command(args ...any) (*ipcResponse, error) {
	p.writeMu.Lock()
	p.reqID++
	id := p.reqID
	p.writeMu.Unlock()

	ch := make(chan ipcResponse, 1)
	p.respMu.Lock()
	p.responses[id] = ch
	p.respMu.Unlock()

	cmd := ipcCommand{Command: args, RequestID: id}
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	p.writeMu.Lock()
	_, err = p.conn.Write(data)
	p.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("writing to mpv: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("mpv error: %s", resp.Error)
		}
		return &resp, nil
	case <-time.After(5 * time.Second):
		p.respMu.Lock()
		delete(p.responses, id)
		p.respMu.Unlock()
		return nil, fmt.Errorf("mpv command timed out")
	}
}

func (p *Player) Play(url string) error {
	_, err := p.command("loadfile", url, "replace")
	return err
}

func (p *Player) TogglePause() error {
	_, err := p.command("cycle", "pause")
	return err
}

func (p *Player) Stop() error {
	_, err := p.command("stop")
	return err
}

func (p *Player) SetVolume(vol int) error {
	_, err := p.command("set_property", "volume", vol)
	return err
}

func (p *Player) GetPosition() (float64, float64, error) {
	posResp, err := p.command("get_property", "time-pos")
	if err != nil {
		return 0, 0, err
	}
	durResp, err := p.command("get_property", "duration")
	if err != nil {
		return 0, 0, err
	}

	pos, _ := posResp.Data.(float64)
	dur, _ := durResp.Data.(float64)
	return pos, dur, nil
}

func (p *Player) Seek(seconds float64) error {
	_, err := p.command("seek", seconds, "relative")
	return err
}

// WaitForEnd blocks until the current track finishes.
func (p *Player) WaitForEnd() {
	<-p.onEnd
}

func (p *Player) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
	os.Remove(socketPath)
}
