package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CreateTarGz(outputPath, sourceDir string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(sourceDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, relPath)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

	return err
}

func ExtractTarGz(archivePath, extractDir string) error {
	// Открываем архив
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Создаем gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Создаем tar reader
	tr := tar.NewReader(gzr)

	// Извлекаем файлы
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // Конец архива
		}
		if err != nil {
			return err
		}

		// Создаем полный путь для файла
		target := filepath.Join(extractDir, header.Name)

		// Проверяем на directory traversal атаки
		if !strings.HasPrefix(target, filepath.Clean(extractDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Создаем директорию
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Создаем файл
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(target)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
