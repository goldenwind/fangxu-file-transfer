package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// The private pipe belongs to the desktop parent. No HTTP control API is exposed.
type desktopSettings struct {
	Directory string `json:"directory"`
	Port      int    `json:"port"`
	Protected bool   `json:"protected"`
	AutoStart bool   `json:"autoStart"`
}

type desktopAddress struct {
	URL    string `json:"url"`
	QRCode string `json:"qrCode"`
}

type desktopStatus struct {
	Settings  desktopSettings  `json:"settings"`
	Running   bool             `json:"running"`
	LocalURL  string           `json:"localUrl"`
	Port      int              `json:"port"`
	Addresses []desktopAddress `json:"addresses"`
	Error     string           `json:"error,omitempty"`
}

type desktopRequest struct {
	Action   string           `json:"action"`
	Settings *desktopSettings `json:"settings,omitempty"`
}

type desktopResponse struct {
	Status desktopStatus `json:"status"`
	Error  string        `json:"error,omitempty"`
}

type desktopController struct {
	settings   desktopSettings
	configPath string
	statePath  string
	handler    *server
	httpServer *http.Server
	claim      *instanceClaim
	done       chan error
	port       int
	lastError  string
}

func loadDesktopSettings(path string) (desktopSettings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return desktopSettings{}, err
	}
	settings := desktopSettings{Directory: filepath.Join(home, "Downloads"), AutoStart: true}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, fmt.Errorf("设置文件损坏，请检查 %s: %w", path, err)
	}
	if settings.Port < 0 || settings.Port > 65535 {
		return settings, errors.New("设置文件中的端口无效")
	}
	return settings, nil
}

func saveDesktopSettings(path string, settings desktopSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (c *desktopController) refresh() {
	if c.done == nil {
		return
	}
	select {
	case err := <-c.done:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			c.lastError = fmt.Sprintf("传输服务已停止: %v", err)
		}
		c.claim.release()
		c.handler, c.httpServer, c.claim, c.done = nil, nil, nil, nil
		c.port = 0
	default:
	}
}

func (c *desktopController) start() error {
	c.refresh()
	if c.httpServer != nil {
		return nil
	}
	handler, err := newServer(c.settings.Directory)
	if err != nil {
		return fmt.Errorf("共享目录不可用，请重新选择: %w", err)
	}
	claim, existing, err := acquireInstance(c.statePath, 3*time.Second)
	if err != nil {
		return err
	}
	if existing != "" {
		return errors.New("已有传输服务在运行，请先关闭原服务后再启动")
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("0.0.0.0", strconv.Itoa(c.settings.Port)))
	if err != nil {
		claim.release()
		return fmt.Errorf("监听端口失败: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := claim.publish(fmt.Sprintf("http://127.0.0.1:%d/", port)); err != nil {
		listener.Close()
		claim.release()
		return err
	}
	handler.instanceToken, handler.desktop = claim.token, true
	handler.setProtected(c.settings.Protected)
	service := &http.Server{Handler: handler.routes(port), ReadHeaderTimeout: 10 * time.Second}
	handler.setShutdown(func() { _ = service.Close() })
	c.handler, c.httpServer, c.claim, c.port = handler, service, claim, port
	c.done = make(chan error, 1)
	done := c.done
	go func() { done <- service.Serve(listener) }()
	c.lastError = ""
	return nil
}

func (c *desktopController) stop() {
	if c.httpServer == nil {
		return
	}
	// Closing connections also cancels in-progress uploads; the upload handler
	// removes temporary files. Waiting for Serve releases the single-instance lock.
	_ = c.httpServer.Close()
	<-c.done
	c.claim.release()
	c.handler, c.httpServer, c.claim, c.done = nil, nil, nil, nil
	c.port = 0
}

func (c *desktopController) configure(settings desktopSettings) error {
	c.refresh()
	if settings.Port < 0 || settings.Port > 65535 {
		return errors.New("端口必须为 0 到 65535，0 表示自动分配")
	}
	if c.httpServer != nil && settings.Port != c.settings.Port {
		return errors.New("请停止服务后再修改端口")
	}
	abs, _, err := resolveDirectory(settings.Directory)
	if err != nil {
		return fmt.Errorf("共享目录不可用: %w", err)
	}
	settings.Directory = abs
	if err := saveDesktopSettings(c.configPath, settings); err != nil {
		return fmt.Errorf("保存设置失败: %w", err)
	}
	if c.handler != nil {
		if err := c.handler.setRoot(abs); err != nil {
			_ = saveDesktopSettings(c.configPath, c.settings)
			return err
		}
		c.handler.setProtected(settings.Protected)
	}
	c.settings = settings
	c.lastError = ""
	return nil
}

func (c *desktopController) status() desktopStatus {
	c.refresh()
	status := desktopStatus{Settings: c.settings, Running: c.httpServer != nil, Port: c.port, Addresses: []desktopAddress{}, Error: c.lastError}
	if !status.Running {
		return status
	}
	status.LocalURL = fmt.Sprintf("http://127.0.0.1:%d/", c.port)
	addresses, err := addressesWithQRCodes(c.port, c.handler.isProtected(), c.handler.token)
	if err != nil {
		status.Error = fmt.Sprintf("生成二维码失败: %v", err)
		return status
	}
	for _, address := range addresses {
		status.Addresses = append(status.Addresses, desktopAddress{URL: address.URL, QRCode: string(address.QRCode)})
	}
	return status
}

func runDesktop(input io.Reader, output io.Writer, configPath string) error {
	if configPath == "" {
		return errors.New("桌面模式需要 --config 设置文件路径")
	}
	settings, err := loadDesktopSettings(configPath)
	if err != nil {
		return err
	}
	statePath, err := serviceStatePath()
	if err != nil {
		return err
	}
	c := &desktopController{settings: settings, configPath: configPath, statePath: statePath}
	defer c.stop()
	if settings.AutoStart {
		if err := c.start(); err != nil {
			c.lastError = err.Error()
		}
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		var request desktopRequest
		err := json.Unmarshal(scanner.Bytes(), &request)
		quit := false
		if err == nil {
			switch request.Action {
			case "status":
			case "start":
				err = c.start()
			case "stop":
				c.stop()
				c.lastError = ""
			case "configure":
				if request.Settings == nil {
					err = errors.New("缺少设置")
				} else {
					err = c.configure(*request.Settings)
				}
			case "quit":
				c.stop()
				quit = true
			default:
				err = errors.New("未知桌面操作")
			}
		}
		response := desktopResponse{Status: c.status()}
		if err != nil {
			response.Error = err.Error()
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
		if quit {
			return nil
		}
	}
	// EOF means the desktop parent closed or crashed. Never leave sharing enabled.
	return scanner.Err()
}
