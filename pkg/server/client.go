package server

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Config SSH连接配置
type Config struct {
	Host                 string        // 服务器地址，如 "192.168.1.100:22"
	Username             string        // 用户名
	Password             string        // 密码（用于普通密码认证或解锁私钥）
	PrivateKeyPath       string        // 私钥文件路径
	PrivateKeyPassphrase string        // 私钥密码（用于解锁加密的私钥）
	Timeout              time.Duration // 连接超时
}

// Client SSH/SFTP客户端
type Client struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     *Config

	mu          sync.Mutex
	termSession *ssh.Session
	termStdin   io.WriteCloser
}

// NewClient 创建新的SSH/SFTP客户端，按优先级尝试认证方式
func NewClient(config *Config) (*Client, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// 尝试顺序：1. 私钥无密码 -> 2. 私钥+密码 -> 3. 普通密码
	var lastErr error

	// ========== 第1层：私钥认证（无密码） ==========
	if config.PrivateKeyPath != "" {
		client, err := tryKeyAuth(config, "")
		if err == nil {
			return client, nil
		}
		lastErr = fmt.Errorf("私钥无密码认证失败: %w", err)

		// 如果错误不是因为需要密码（比如文件不存在），直接返回
		if !isPassphraseRequired(err) {
			return nil, lastErr
		}

		// ========== 第2层：私钥 + 密码短语 ==========
		if config.PrivateKeyPassphrase != "" {
			client, err = tryKeyAuth(config, config.PrivateKeyPassphrase)
			if err == nil {
				return client, nil
			}
			lastErr = fmt.Errorf("私钥+密码短语认证失败: %w", err)
		}
	}

	// ========== 第3层：普通密码认证 ==========
	if config.Password != "" {
		client, err := tryPasswordAuth(config)
		if err == nil {
			return client, nil
		}
		lastErr = fmt.Errorf("密码认证失败: %w", err)
	}

	return nil, fmt.Errorf("所有认证方式均失败，最后的错误: %w", lastErr)
}

// tryKeyAuth 尝试密钥认证，passphrase可为空
func tryKeyAuth(config *Config, passphrase string) (*Client, error) {
	key, err := os.ReadFile(config.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	var signer ssh.Signer
	if passphrase == "" {
		signer, err = ssh.ParsePrivateKey(key)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
	}

	if err != nil {
		return nil, err // 返回原始错误，让上层判断是否需要密码
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         config.Timeout,
	}

	return dialAndCreateClient(config, sshConfig)
}

// tryPasswordAuth 尝试密码认证（包含键盘交互式作为备选）
func tryPasswordAuth(config *Config) (*Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: config.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.Password),
			// 某些服务器需要键盘交互式而不是纯密码
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = config.Password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         config.Timeout,
	}

	return dialAndCreateClient(config, sshConfig)
}

// dialAndCreateClient 建立连接并创建SFTP客户端
func dialAndCreateClient(config *Config, sshConfig *ssh.ClientConfig) (*Client, error) {
	sshClient, err := ssh.Dial("tcp", config.Host, sshConfig)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("SFTP连接失败: %w", err)
	}

	return &Client{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		config:     config,
	}, nil
}

// isPassphraseRequired 检查错误是否因为私钥需要密码
func isPassphraseRequired(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 匹配常见的需要密码短语的错误信息
	return strings.Contains(errStr, "cannot decode encrypted private keys") ||
		strings.Contains(errStr, "password protected") ||
		strings.Contains(errStr, "passphrase") ||
		strings.Contains(errStr, "decrypt")
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	if c.termSession != nil {
		_ = c.termSession.Close()
		c.termSession = nil
	}
	c.mu.Unlock()

	if c.sftpClient != nil {
		_ = c.sftpClient.Close()
	}
	if c.sshClient != nil {
		_ = c.sshClient.Close()
	}
	return nil
}

// GetSFTPClient 获取SFTP客户端
func (c *Client) GetSFTPClient() *sftp.Client {
	return c.sftpClient
}

// GetSSHClient 获取SSH客户端
func (c *Client) GetSSHClient() *ssh.Client {
	return c.sshClient
}

// UploadFile 上传文件到服务器
func (c *Client) UploadFile(localPath, remotePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer src.Close()

	remoteDir := filepath.Dir(remotePath)
	if err := c.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("创建远程目录失败: %w", err)
	}

	dst, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	return nil
}

// MkdirAll 递归创建目录
func (c *Client) MkdirAll(path string) error {
	return c.sftpClient.MkdirAll(path)
}

// ExecuteCommand 执行非交互式命令，返回stdout+stderr
func (c *Client) ExecuteCommand(cmd string) (string, error) {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf
	if err := session.Run(cmd); err != nil {
		return buf.String(), err
	}
	return buf.String(), nil
}

// RunInteractiveSession 启动交互式shell
func (c *Client) RunInteractiveSession(stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}

	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		session.Close()
		return fmt.Errorf("请求PTY失败: %w", err)
	}

	termIn, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("获取stdin失败: %w", err)
	}
	session.Stdout = stdout
	session.Stderr = stderr

	c.mu.Lock()
	c.termSession = session
	c.termStdin = termIn
	c.mu.Unlock()

	if err := session.Shell(); err != nil {
		session.Close()
		return fmt.Errorf("启动shell失败: %w", err)
	}

	go func() {
		io.Copy(termIn, stdin)
		_ = termIn.Close()
	}()

	err = session.Wait()

	c.mu.Lock()
	c.termSession = nil
	c.termStdin = nil
	c.mu.Unlock()

	return err
}

// ResizePTY 调整当前活跃交互式PTY窗口大小
func (c *Client) ResizePTY(cols, rows int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.termSession == nil {
		return nil
	}
	return c.termSession.WindowChange(rows, cols)
}

// SendInput 向当前交互式会话发送输入
func (c *Client) SendInput(input string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.termStdin == nil {
		return fmt.Errorf("没有活跃的交互式会话")
	}
	_, err := c.termStdin.Write([]byte(input))
	return err
}
