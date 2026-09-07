package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	comfyInstallAuto     = "auto"
	comfyInstallPortable = "portable"
	comfyInstallDesktop  = "desktop"
	defaultComfyPort     = 8188
)

var comfyControlMu sync.Mutex
var managedComfy *os.Process
var managedComfyStartedAt time.Time
var managedComfyCommand string

type comfyInstallInfo struct {
	InstallDir  string
	InstallType string
	Launcher    string
}

func normalizeComfyInstallType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == comfyInstallPortable || normalized == comfyInstallDesktop {
		return normalized
	}
	return comfyInstallAuto
}

func normalizeComfyPort(value any, fallback int) int {
	port := int(numberValue(value))
	if port <= 0 || port > 65535 {
		return fallback
	}
	return port
}

func detectComfyInstall(options bridgeOptions) comfyInstallInfo {
	installDir := strings.TrimSpace(options.ComfyInstallDir)
	requested := normalizeComfyInstallType(options.ComfyInstallType)
	info := comfyInstallInfo{InstallDir: installDir, InstallType: requested}
	if installDir != "" {
		if requested == comfyInstallAuto || requested == comfyInstallPortable {
			if launcher := detectPortableLauncher(installDir); launcher != "" {
				return comfyInstallInfo{InstallDir: installDir, InstallType: comfyInstallPortable, Launcher: launcher}
			}
		}
		if requested == comfyInstallAuto || requested == comfyInstallDesktop {
			if launcher := detectDesktopLauncher(installDir); launcher != "" {
				return comfyInstallInfo{InstallDir: installDir, InstallType: comfyInstallDesktop, Launcher: launcher}
			}
		}
	}
	if requested == comfyInstallPortable {
		for _, dir := range commonPortableComfyDirs() {
			if launcher := detectPortableLauncher(dir); launcher != "" {
				return comfyInstallInfo{InstallDir: dir, InstallType: comfyInstallPortable, Launcher: launcher}
			}
		}
		return info
	}
	if requested == comfyInstallDesktop {
		for _, dir := range commonDesktopComfyDirs() {
			if launcher := detectDesktopLauncher(dir); launcher != "" {
				return comfyInstallInfo{InstallDir: dir, InstallType: comfyInstallDesktop, Launcher: launcher}
			}
		}
		return info
	}
	for _, dir := range commonPortableComfyDirs() {
		if launcher := detectPortableLauncher(dir); launcher != "" {
			return comfyInstallInfo{InstallDir: dir, InstallType: comfyInstallPortable, Launcher: launcher}
		}
	}
	for _, dir := range commonDesktopComfyDirs() {
		if launcher := detectDesktopLauncher(dir); launcher != "" {
			return comfyInstallInfo{InstallDir: dir, InstallType: comfyInstallDesktop, Launcher: launcher}
		}
	}
	return info
}

func commonPortableComfyDirs() []string {
	if runtime.GOOS == "windows" {
		return []string{"D:\\ComfyUI", "C:\\ComfyUI", filepath.Join(homeDirOrEmpty(), "ComfyUI")}
	}
	return []string{filepath.Join(homeDirOrEmpty(), "ComfyUI"), "/opt/ComfyUI", "/usr/local/ComfyUI"}
}

func commonDesktopComfyDirs() []string {
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(localAppDataOrEmpty(), "Programs", "@comfyorgcomfyui-electron")}
	}
	if runtime.GOOS == "darwin" {
		return []string{"/Applications", filepath.Join(homeDirOrEmpty(), "Applications")}
	}
	return nil
}

func detectPortableLauncher(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	candidates := []string{"run_nvidia_gpu.bat", "run_cpu.bat"}
	if runtime.GOOS != "windows" {
		candidates = []string{"run_nvidia_gpu.sh", "run_cpu.sh"}
	}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func detectDesktopLauncher(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		for _, name := range []string{"Comfy Desktop.exe", "ComfyUI.exe"} {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
		return ""
	}
	if runtime.GOOS == "darwin" {
		appPath := dir
		if !strings.HasSuffix(strings.ToLower(appPath), ".app") {
			appPath = filepath.Join(appPath, "ComfyUI.app")
		}
		if info, err := os.Stat(appPath); err == nil && info.IsDir() {
			binaryPath := filepath.Join(appPath, "Contents", "MacOS", "ComfyUI")
			if binary, err := os.Stat(binaryPath); err == nil && !binary.IsDir() {
				return binaryPath
			}
		}
	}
	return ""
}

func probeComfyPort(value string) (int, bool) {
	base, err := parseComfyBaseURL(value)
	if err != nil {
		return 0, false
	}
	ports := []int{defaultComfyPort, 8189, 8190, 8191, 8288}
	if preferred := explicitComfyPort(value); preferred > 0 {
		ports = append([]int{preferred}, ports...)
	}
	for _, port := range ports {
		candidate := fmt.Sprintf("%s:%d", base, port)
		if comfyURLAvailable(candidate) {
			return port, true
		}
	}
	return 0, false
}

func comfyURLAvailable(url string) bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	response, err := client.Get(url + "/system_stats")
	if err != nil {
		return false
	}
	_ = response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func parseComfyBaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	index := strings.Index(value, "://")
	if index <= 0 {
		return "", errors.New("ComfyUI 地址必须是完整 HTTP/HTTPS 地址")
	}
	scheme, rest := value[:index], value[index+3:]
	if pathIndex := strings.IndexAny(rest, "/?#"); pathIndex >= 0 {
		rest = rest[:pathIndex]
	}
	if index := strings.LastIndex(rest, ":"); index >= 0 {
		rest = rest[:index]
	}
	if rest == "" {
		return "", errors.New("ComfyUI 地址缺少主机")
	}
	return scheme + "://" + rest, nil
}

func explicitComfyPort(value string) int {
	parsed, err := parseComfyBaseURL(value)
	if err != nil {
		return 0
	}
	hostPort := strings.TrimPrefix(strings.TrimPrefix(parsed, "http://"), "https://")
	index := strings.LastIndex(hostPort, ":")
	if index < 0 {
		return 0
	}
	port, err := strconv.Atoi(hostPort[index+1:])
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}

func startComfy(options bridgeOptions) (jsonMap, error) {
	comfyControlMu.Lock()
	defer comfyControlMu.Unlock()
	info := detectComfyInstall(options)
	if info.Launcher == "" {
		return detectResult(options, info, false), errors.New("未找到 ComfyUI 启动器；请确认安装目录或安装类型")
	}
	if managedComfyAlive() {
		return detectResult(options, info, true), nil
	}
	port := normalizeComfyPort(options.ComfyPort, defaultComfyPort)
	base, _ := parseComfyBaseURL(options.Comfy)
	candidate := fmt.Sprintf("%s:%d", base, port)
	if comfyURLAvailable(candidate) {
		return detectResult(options, info, false), nil
	}
	command, process, err := launchComfy(info.Launcher, info.InstallType, port)
	if err != nil {
		return detectResult(options, info, false), fmt.Errorf("启动 ComfyUI 失败：%w", err)
	}
	managedComfy = process
	managedComfyStartedAt = time.Now()
	managedComfyCommand = command
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if comfyURLAvailable(candidate) {
			if !options.ComfyExplicit || options.Comfy == "http://127.0.0.1:8188" {
				options.Comfy = candidate
				if err := saveBridgeOptions(options); err != nil {
					return detectResult(options, info, true), err
				}
			}
			return jsonMap{
				"installDir":  info.InstallDir,
				"installType": info.InstallType,
				"comfyUrl":    candidate,
				"comfyPort":   port,
				"comfyOnline": true,
				"managed":     true,
			}, nil
		}
		time.Sleep(1200 * time.Millisecond)
	}
	return detectResult(options, info, true), errors.New("ComfyUI 已发出启动命令，但端口在 45 秒内未就绪")
}

func stopComfy(options bridgeOptions) (jsonMap, error) {
	comfyControlMu.Lock()
	defer comfyControlMu.Unlock()
	info := detectComfyInstall(options)
	if managedComfy == nil {
		return detectResult(options, info, false), errors.New("ComfyUI 不是由 Bridge 启动的，Bridge 不会停止它")
	}
	process := managedComfy
	managedComfy = nil
	if err := terminateComfyProcess(process); err != nil {
		return detectResult(options, info, false), fmt.Errorf("停止 ComfyUI 失败：%w", err)
	}
	return jsonMap{
		"installDir":  info.InstallDir,
		"installType": info.InstallType,
		"comfyOnline": false,
		"managed":     false,
	}, nil
}

func detectComfy(options bridgeOptions) (jsonMap, error) {
	info := detectComfyInstall(options)
	url, port := detectComfyURL(options)
	return jsonMap{
		"installDir":  info.InstallDir,
		"installType": info.InstallType,
		"launcher":    info.Launcher,
		"comfyUrl":    url,
		"comfyPort":   port,
		"comfyOnline": comfyURLAvailable(url),
		"managed":     managedComfyAlive(),
	}, nil
}

func detectComfyURL(options bridgeOptions) (string, int) {
	if _, port, online := probeComfyEndpoint(options.Comfy); online {
		return options.Comfy, port
	}
	info := detectComfyInstall(options)
	if info.Launcher != "" {
		port := normalizeComfyPort(options.ComfyPort, defaultComfyPort)
		base, _ := parseComfyBaseURL(options.Comfy)
		return fmt.Sprintf("%s:%d", base, port), port
	}
	return options.Comfy, 0
}

func probeComfyEndpoint(value string) (string, int, bool) {
	port, ok := probeComfyPort(value)
	if !ok {
		return value, 0, false
	}
	base, _ := parseComfyBaseURL(value)
	return fmt.Sprintf("%s:%d", base, port), port, true
}

func detectResult(options bridgeOptions, info comfyInstallInfo, managed bool) jsonMap {
	port := normalizeComfyPort(options.ComfyPort, defaultComfyPort)
	base, _ := parseComfyBaseURL(options.Comfy)
	url := fmt.Sprintf("%s:%d", base, port)
	return jsonMap{
		"installDir":  info.InstallDir,
		"installType": info.InstallType,
		"comfyUrl":    url,
		"comfyPort":   port,
		"comfyOnline": comfyURLAvailable(url),
		"managed":     managed,
	}
}

func managedComfyAlive() bool {
	return managedComfy != nil
}

func launchComfy(launcher string, installType string, port int) (string, *os.Process, error) {
	var command *exec.Cmd
	if installType == comfyInstallPortable {
		if runtime.GOOS == "windows" {
			command = exec.Command("cmd", "/C", filepath.Base(launcher), "--port", strconv.Itoa(port))
		} else {
			command = exec.Command("sh", "-c", fmt.Sprintf("'%s' --port %d", strings.ReplaceAll(launcher, "'", `'"'"'`), port))
		}
	} else {
		command = exec.Command(launcher)
	}
	command.Dir = filepath.Dir(launcher)
	command.Env = append(os.Environ(), "COMFYUI_PORT="+strconv.Itoa(port))
	configureComfyProcess(command)
	if err := command.Start(); err != nil {
		return "", nil, err
	}
	return command.String(), command.Process, nil
}

func localAppDataOrEmpty() string {
	return strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
}

func homeDirOrEmpty() string {
	home, _ := os.UserHomeDir()
	return home
}
