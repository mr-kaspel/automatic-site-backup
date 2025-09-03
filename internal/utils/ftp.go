package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// FTPClient interface for interaction with servers
type FTPClient interface {
	Close() error
	ListFiles(directory string) ([]RemoteFile, error)
	DownloadFile(remotePath, localPath string) error
	UploadFile(localPath, remotePath string) error
	DeleteFile(remotePath string) error
	HashFile(remotePath string) (string, error)
	CurrentDir() (string, error)
	IsConnected() bool
	Reconnect() error
}

// RemoteFile represents a file on a remote server
type RemoteFile struct {
	Name string
	Size int64
}

// implementation of an FTP client
type MyFTPClient struct {
	conn     *ftp.ServerConn
	host     string
	user     string
	password string
	port     string
}

// NewFTPClient creates a new FTP client
func NewFTPClient(host, user, password, port string) (FTPClient, error) {
	if port == "" {
		port = "21"
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	// Create FTP connection with timeout
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to FTP server %s: %v", addr, err)
	}

	err = conn.Login(user, password)
	if err != nil {
		conn.Quit() // Close connection on login failure
		return nil, fmt.Errorf("failed to login to FTP server: %v", err)
	}

	return &MyFTPClient{
		conn:     conn,
		host:     host,
		user:     user,
		password: password,
		port:     port,
	}, nil
}

func (f *MyFTPClient) Close() error {
	return f.conn.Quit()
}

func (f *MyFTPClient) ListFiles(directory string) ([]RemoteFile, error) {
	var allFiles []RemoteFile

	// Helper function for recursive traversal
	var walkDir func(string) error
	walkDir = func(dir string) error {
		return executeWithReconnect(f, func() error {
			fmt.Printf("Attempting to list directory: %s\n", dir)
			entries, err := f.conn.List(dir)
			if err != nil {
				fmt.Printf("Failed to list directory %s: %v\n", dir, err)
				return err
			}

			for _, entry := range entries {
				if entry.Type == ftp.EntryTypeFile {
					// Use proper path joining for cross-platform compatibility
					filePath := filepath.Join(dir, entry.Name)
					// Normalize path separators for consistency
					filePath = filepath.ToSlash(filePath)
					allFiles = append(allFiles, RemoteFile{
						Name: filePath,
						Size: int64(entry.Size),
					})
				} else if entry.Type == ftp.EntryTypeFolder {
					// Skip hidden directories and common system directories
					if entry.Name == "." || entry.Name == ".." || strings.HasPrefix(entry.Name, ".") {
						continue
					}
					newDir := filepath.Join(dir, entry.Name)
					fmt.Printf("Entering folder: %s\n", newDir)
					err := walkDir(newDir)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
	}

	// Start traversal from the root directory
	err := walkDir(directory)
	if err != nil {
		return nil, err
	}

	return allFiles, nil
}

func (f *MyFTPClient) DownloadFile(remotePath, localPath string) error {
	return executeWithReconnect(f, func() error {
		resp, err := f.conn.Retr(remotePath)
		if err != nil {
			return err
		}
		defer resp.Close()

		localFile, err := os.Create(localPath)
		if err != nil {
			return err
		}
		defer localFile.Close()

		_, err = io.Copy(localFile, resp)
		return err
	})
}

func (f *MyFTPClient) HashFile(remotePath string) (string, error) {
	var result string
	err := executeWithReconnect(f, func() error {
		resp, err := f.conn.Retr(remotePath)
		if err != nil {
			return err
		}
		defer resp.Close()

		hasher := sha256.New()
		_, err = io.Copy(hasher, resp)
		if err != nil {
			return err
		}

		result = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	return result, err
}

func (f *MyFTPClient) CurrentDir() (string, error) {
	var result string
	err := executeWithReconnect(f, func() error {
		dir, err := f.conn.CurrentDir()
		if err != nil {
			return err
		}
		result = dir
		return nil
	})
	return result, err
}

func (f *MyFTPClient) UploadFile(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	return executeWithReconnect(f, func() error {
		// FTP Stor method takes (remotePath, reader) parameters
		return f.conn.Stor(remotePath, localFile)
	})
}

func (f *MyFTPClient) DeleteFile(remotePath string) error {
	return executeWithReconnect(f, func() error {
		return f.conn.Delete(remotePath)
	})
}

func (f *MyFTPClient) IsConnected() bool {
	if f.conn == nil {
		return false
	}
	// Try to get current directory to check connection
	_, err := f.conn.CurrentDir()
	return err == nil
}

func (f *MyFTPClient) Reconnect() error {
	// Close existing connection if any
	if f.conn != nil {
		f.conn.Quit()
	}

	// Create new connection
	addr := fmt.Sprintf("%s:%s", f.host, f.port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("failed to reconnect to FTP server %s: %v", addr, err)
	}

	err = conn.Login(f.user, f.password)
	if err != nil {
		conn.Quit()
		return fmt.Errorf("failed to login to FTP server: %v", err)
	}

	f.conn = conn
	return nil
}

// implementing an SFTP client
type MySFTPClient struct {
	client   *sftp.Client
	conn     *ssh.Client
	host     string
	user     string
	password string
	port     string
}

// NewSFTPClient creates a new SFTP client
func NewSFTPClient(host, user, password, port string) (FTPClient, error) {
	if port == "" {
		port = "22"
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server %s: %v", addr, err)
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close() // Close SSH connection if SFTP client creation fails
		return nil, fmt.Errorf("failed to create SFTP client: %v", err)
	}

	return &MySFTPClient{
		client:   client,
		conn:     conn,
		host:     host,
		user:     user,
		password: password,
		port:     port,
	}, nil
}

func (s *MySFTPClient) Close() error {
	// Close SFTP client first
	if err := s.client.Close(); err != nil {
		// Log error but continue to close SSH connection
		fmt.Printf("Warning: Failed to close SFTP client: %v\n", err)
	}

	// Close SSH connection
	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("failed to close SSH connection: %v", err)
	}

	return nil
}

func (s *MySFTPClient) ListFiles(directory string) ([]RemoteFile, error) {
	var allFiles []RemoteFile

	// Helper function for recursive traversal
	var walkDir func(string) error
	walkDir = func(dir string) error {
		return executeWithReconnect(s, func() error {
			files, err := s.client.ReadDir(dir)
			if err != nil {
				return err
			}

			for _, file := range files {
				if file.IsDir() {
					// Skip hidden directories and common system directories
					if file.Name() == "." || file.Name() == ".." || strings.HasPrefix(file.Name(), ".") {
						continue
					}
					// Recursively traversing folders
					err := walkDir(filepath.Join(dir, file.Name()))
					if err != nil {
						return err
					}
				} else {
					// Add files to the list
					filePath := filepath.Join(dir, file.Name())
					// Normalize path separators for consistency
					filePath = filepath.ToSlash(filePath)
					allFiles = append(allFiles, RemoteFile{
						Name: filePath,
						Size: file.Size(),
					})
				}
			}
			return nil
		})
	}

	// We start from the specified directory
	err := walkDir(directory)
	if err != nil {
		return nil, err
	}

	return allFiles, nil
}

func (s *MySFTPClient) DownloadFile(remotePath, localPath string) error {
	return executeWithReconnect(s, func() error {
		srcFile, err := s.client.Open(remotePath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(localPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func (s *MySFTPClient) HashFile(remotePath string) (string, error) {
	var result string
	err := executeWithReconnect(s, func() error {
		srcFile, err := s.client.Open(remotePath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		hasher := sha256.New()
		_, err = io.Copy(hasher, srcFile)
		if err != nil {
			return err
		}

		result = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	return result, err
}

func (s *MySFTPClient) CurrentDir() (string, error) {
	var result string
	err := executeWithReconnect(s, func() error {
		dir, err := s.client.Getwd()
		if err != nil {
			return err
		}
		result = dir
		return nil
	})
	return result, err
}

func (s *MySFTPClient) UploadFile(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	return executeWithReconnect(s, func() error {
		// Create remote directory if it doesn't exist
		remoteDir := filepath.Dir(remotePath)
		if remoteDir != "." && remoteDir != "/" {
			err = s.client.MkdirAll(remoteDir)
			if err != nil {
				// Ignore error if directory already exists
				fmt.Printf("Warning: Could not create remote directory %s: %v\n", remoteDir, err)
			}
		}

		remoteFile, err := s.client.Create(remotePath)
		if err != nil {
			return err
		}
		defer remoteFile.Close()

		_, err = io.Copy(remoteFile, localFile)
		return err
	})
}

func (s *MySFTPClient) DeleteFile(remotePath string) error {
	return executeWithReconnect(s, func() error {
		return s.client.Remove(remotePath)
	})
}

func (s *MySFTPClient) IsConnected() bool {
	if s.client == nil || s.conn == nil {
		return false
	}
	// Try to get current working directory to check connection
	_, err := s.client.Getwd()
	return err == nil
}

func (s *MySFTPClient) Reconnect() error {
	// Close existing connections if any
	if s.client != nil {
		s.client.Close()
	}
	if s.conn != nil {
		s.conn.Close()
	}

	// Create new SSH connection
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to reconnect to SSH server %s: %v", addr, err)
	}

	// Create new SFTP client
	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	s.client = client
	s.conn = conn
	return nil
}

// executeWithReconnect executes a function with automatic reconnection on failure
func executeWithReconnect(client FTPClient, operation func() error) error {
	err := operation()
	if err != nil {
		// Check if connection is still alive
		if !client.IsConnected() {
			fmt.Printf("Connection lost, attempting to reconnect...\n")
			reconnectErr := client.Reconnect()
			if reconnectErr != nil {
				return fmt.Errorf("failed to reconnect: %v", reconnectErr)
			}
			fmt.Printf("Reconnected successfully, retrying operation...\n")
			// Retry the operation once after reconnection
			return operation()
		}
	}
	return err
}
