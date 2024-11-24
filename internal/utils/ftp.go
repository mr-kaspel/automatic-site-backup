package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// FTPClient интерфейс для взаимодействия с серверами
type FTPClient interface {
	Close() error
	ListFiles(directory string) ([]RemoteFile, error)
	DownloadFile(remotePath, localPath string) error
	HashFile(remotePath string) (string, error)
}

// RemoteFile представляет файл на удалённом сервере
type RemoteFile struct {
	Name string
	Size int64
}

// implementation of an FTP client
type MyFTPClient struct {
	conn *ftp.ServerConn
}

// NewFTPClient creates a new FTP client
func NewFTPClient(host, user, password string) (FTPClient, error) {
	conn, err := ftp.Dial(host)
	if err != nil {
		return nil, err
	}

	err = conn.Login(user, password)
	if err != nil {
		return nil, err
	}

	return &MyFTPClient{conn: conn}, nil
}

func (f *MyFTPClient) Close() error {
	return f.conn.Quit()
}

func (f *MyFTPClient) ListFiles(directory string) ([]RemoteFile, error) {
	entries, err := f.conn.List(directory)
	if err != nil {
		return nil, err
	}

	var files []RemoteFile
	for _, entry := range entries {
		if entry.Type == ftp.EntryTypeFile {
			files = append(files, RemoteFile{
				Name: entry.Name,
				Size: int64(entry.Size),
			})
		}
	}
	return files, nil
}

func (f *MyFTPClient) DownloadFile(remotePath, localPath string) error {
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
}

func (f *MyFTPClient) HashFile(remotePath string) (string, error) {
	resp, err := f.conn.Retr(remotePath)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, resp)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// implementing an SFTP client
type MySFTPClient struct {
	client *sftp.Client
}

// NewSFTPClient creates a new SFTP client
func NewSFTPClient(host, user, password string) (FTPClient, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return &MySFTPClient{client: client}, nil
}

func (s *MySFTPClient) Close() error {
	return s.client.Close()
}

func (s *MySFTPClient) ListFiles(directory string) ([]RemoteFile, error) {
	files, err := s.client.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	var remoteFiles []RemoteFile
	for _, file := range files {
		if !file.IsDir() {
			remoteFiles = append(remoteFiles, RemoteFile{
				Name: file.Name(),
				Size: file.Size(),
			})
		}
	}
	return remoteFiles, nil
}

func (s *MySFTPClient) DownloadFile(remotePath, localPath string) error {
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
}

func (s *MySFTPClient) HashFile(remotePath string) (string, error) {
	srcFile, err := s.client.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, srcFile)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
