package server

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	mu sync.Mutex
}

// TerminalSession 独立的终端会话
type TerminalSession struct {
	Session *ssh.Session
	Stdin   io.WriteCloser
}

// Resize 调整终端大小
func (ts *TerminalSession) Resize(cols, rows int) error {
	if ts.Session == nil {
		return fmt.Errorf("会话未初始化")
	}
	return ts.Session.WindowChange(rows, cols)
}

// Close 关闭终端会话
func (ts *TerminalSession) Close() error {
	if ts.Session != nil {
		return ts.Session.Close()
	}
	return nil
}

// NewClient 创建新的SSH/SFTP客户端，尝试所有配置的认证方式
func NewClient(config *Config) (*Client, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	var authMethods []ssh.AuthMethod

	// 1. 尝试加载私钥认证
	if config.PrivateKeyPath != "" {
		key, err := os.ReadFile(config.PrivateKeyPath)
		if err == nil {
			// 去除可能的空白字符
			key = bytes.TrimSpace(key)
			var signer ssh.Signer

			// 优先尝试带密码解析（如果配置了密码）
			if config.PrivateKeyPassphrase != "" {
				s, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(config.PrivateKeyPassphrase))
				if err == nil {
					signer = s
				}
			}

			// 如果signer仍为空（没配置密码，或带密码解析失败），尝试无密码解析
			if signer == nil {
				s, err := ssh.ParsePrivateKey(key)
				if err == nil {
					signer = s
				}
			}

			if signer != nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	// 2. 添加密码认证（普通密码 ）
	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))

	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未配置任何有效的认证方式 (私钥解析失败且无密码)")
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         config.Timeout,
		// 增加对旧版加密算法的支持，以防服务器较旧或算法不匹配
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoRSA,
			ssh.KeyAlgoDSA,
			ssh.KeyAlgoECDSA256,
			ssh.KeyAlgoECDSA384,
			ssh.KeyAlgoECDSA521,
			ssh.KeyAlgoED25519,
		},
	}

	return dialAndCreateClient(config, sshConfig)
}

// dialAndCreateClient 建立连接并创建SFTP客户端
func dialAndCreateClient(config *Config, sshConfig *ssh.ClientConfig) (*Client, error) {
	sshClient, err := ssh.Dial("tcp", config.Host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
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

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	defer func() {
		if err := session.Close(); err != nil && err != io.EOF {
			// 仅记录非 EOF 错误，或者直接忽略
		}
	}()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf
	if err := session.Run(cmd); err != nil {
		return buf.String(), err
	}
	return buf.String(), nil
}

// NewTerminalSession 创建一个新的终端会话
func (c *Client) NewTerminalSession(stdin io.Reader, stdout, stderr io.Writer, cols, rows int) (*TerminalSession, error) {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	// 如果未指定大小，使用默认值
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	if err := session.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		session.Close()
		return nil, fmt.Errorf("请求PTY失败: %w", err)
	}

	termIn, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("获取stdin失败: %w", err)
	}
	session.Stdout = stdout
	session.Stderr = stderr

	if err := session.Shell(); err != nil {
		session.Close()
		return nil, fmt.Errorf("启动shell失败: %w", err)
	}

	go func() {
		_, _ = io.Copy(termIn, stdin)
		_ = termIn.Close()
	}()

	return &TerminalSession{
		Session: session,
		Stdin:   termIn,
	}, nil
}
