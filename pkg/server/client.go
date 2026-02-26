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
	Password             string        // 密码
	PrivateKeyPath       string        // 私钥文件路径
	PrivateKeyPassphrase string        // 私钥密码
	Timeout              time.Duration // 连接超时
}

// Client SSH/SFTP客户端，支持执行命令和交互式终端
type Client struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     *Config

	mu          sync.Mutex     // 保护 termSession
	termSession *ssh.Session   // 当前活跃交互式 session
	termStdin   io.WriteCloser // 交互式 stdin，用于外部写入
}

// NewClient 创建新的SSH/SFTP客户端
func NewClient(config *Config) (*Client, error) {
	authMethods := []ssh.AuthMethod{}

	// 尝试使用私钥认证
	if config.PrivateKeyPath != "" {
		key, err := os.ReadFile(config.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("无法读取私钥文件: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// 如果私钥解析失败，尝试使用密码解析
			if config.PrivateKeyPassphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(config.PrivateKeyPassphrase))
				if err != nil {
					return nil, fmt.Errorf("无法使用私钥密码解析私钥: %w", err)
				}
			} else {
				return nil, fmt.Errorf("无法解析私钥: %w", err)
			}
		}
		//将密钥认证添加到认证方法里
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// 如果提供了密码，则添加密码认证
	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	// 如果没有提供任何认证方式，则返回错误
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未提供SSH认证方式（密码或私钥）")
	}

	//配置好ssh用于连接的config
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods, // 使用组合的认证方法
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         config.Timeout,
	}

	sshClient, err := ssh.Dial("tcp", config.Host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
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
		return "", err
	}
	return buf.String(), nil
}

// RunInteractiveSession 启动交互式shell，将stdin/stdout/stderr与调用方的pipe连接。
// 该方法阻塞直到远端会话结束。
func (c *Client) RunInteractiveSession(stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}

	// 请求PTY
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		session.Close()
		return fmt.Errorf("请求PTY失败: %w", err)
	}

	// 管道
	termIn, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("获取stdin失败: %w", err)
	}
	session.Stdout = stdout
	session.Stderr = stderr

	// 更新活跃session引用
	c.mu.Lock()
	c.termSession = session
	c.termStdin = termIn
	c.mu.Unlock()

	// 启动shell
	if err := session.Shell(); err != nil {
		session.Close()
		return fmt.Errorf("启动shell失败: %w", err)
	}

	// 异步复制stdin
	go func() {
		io.Copy(termIn, stdin)
		_ = termIn.Close()
	}()

	// 阻塞直到退出
	err = session.Wait()

	// 清理引用
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
