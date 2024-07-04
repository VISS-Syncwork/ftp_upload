package main

import (
	"archive/tar"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jlaffaye/ftp"
)

func main() {
	ftpHost := flag.String("host", "", "FTP-Host-Adresse")
	ftpPort := flag.Int("port", 21, "FTP-Port")
	ftpUser := flag.String("user", "ftpuser", "FTP-Benutzername")
	ftpPass := flag.String("pass", "password", "FTP-Passwort")
	ftpDestPath := flag.String("path", "/uploads", "Zielverzeichnis auf dem FTP-Server")
	sourceDir := flag.String("dir", ".", "Quellverzeichnis, das gepackt und hochgeladen werden soll")
	flag.Parse()

	tarFileName := "archive.tar"

	sourceDirAbs, err := filepath.Abs(*sourceDir)
	if err != nil {
		fmt.Println("Fehler beim Ermitteln des absoluten Pfads:", err)
		return
	}

	err = createTarArchive(sourceDirAbs, tarFileName)
	if err != nil {
		fmt.Println("Fehler beim Erstellen des Tar-Archivs:", err)
		return
	}

	// FTP-Verbindung herstellen mit SSL und ohne Zertifikatsprüfung
	ftpURL := fmt.Sprintf("%s:%d", *ftpHost, *ftpPort)
	tlsCOnfig := &tls.Config{InsecureSkipVerify: true}
	dialOption := ftp.DialWithExplicitTLS(tlsCOnfig)
	conn, err := ftp.Dial(ftpURL, dialOption)

	if err != nil {
		fmt.Println("Fehler beim Verbinden mit dem FTP-Server:", err)
		return
	}
	defer conn.Quit()

	err = conn.Login(*ftpUser, *ftpPass)
	if err != nil {
		fmt.Println("Fehler beim Anmelden beim FTP-Server:", err)
		return
	}

	err = uploadFile(conn, tarFileName, *ftpDestPath)
	if err != nil {
		fmt.Println("Fehler beim Hochladen des Tar-Archivs:", err)
		return
	}

	fmt.Println("Erfolgreich gepackt und hochgeladen:", tarFileName)
}

func createTarArchive(sourceDir, tarFileName string) error {
	tarFile, err := os.Create(tarFileName)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()

	filepath.Walk(sourceDir, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fileInfo, fileInfo.Name())
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		if fileInfo.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		fmt.Printf("Packing: %s\n", header.Name)
		return nil
	})

	return err
}

func uploadFile(conn *ftp.ServerConn, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := conn.ChangeDir(remotePath); err != nil {
		if strings.HasPrefix(err.Error(), "550") {
			err := conn.MakeDir(remotePath)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	info, _ := file.Stat()
	progress := &ProgressReader{Reader: file, Total: info.Size(), Callback: func(uploaded int64) {
		fmt.Printf("Uploading: %d / %d bytes\n", uploaded, info.Size())
	}}

	return conn.Stor(filepath.Base(localPath), progress)
}

type ProgressReader struct {
	Reader   io.Reader
	Total    int64
	Uploaded int64
	Callback func(uploaded int64)
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Uploaded += int64(n)
	pr.Callback(pr.Uploaded)
	return
}
