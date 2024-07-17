package main

import (
	"archive/tar"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/jlaffaye/ftp"
)

func main() {
	ftpServer := flag.String("host", "", "FTP-Host-Adresse")
	ftpUser := flag.String("user", "ftpuser", "FTP-Benutzername")
	ftpPassword := flag.String("pass", "password", "FTP-Passwort")
	localDir := flag.String("dir", ".", "Quellverzeichnis, das gepackt und hochgeladen werden soll")
	remoteFile := "remote_archive.tar"

	// Create an FTP connection
	ftpConn, err := ftp.Dial(*ftpServer+":21", ftp.DialWithTLS(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		log.Fatal(err)
	}

	// Login to FTP server
	err = ftpConn.Login(*ftpUser, *ftpPassword)
	if err != nil {
		log.Fatal(err)
	}
	defer ftpConn.Quit()

	// Create a pipe to stream the data
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		tw := tar.NewWriter(pw)
		defer tw.Close()

		err := filepath.Walk(*localDir, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if fi.IsDir() {
				return nil
			}

			hdr, err := tar.FileInfoHeader(fi, file)
			if err != nil {
				return err
			}

			// Update the header to use a relative path
			hdr.Name, _ = filepath.Rel(*localDir, file)

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			log.Fatal(err)
		}
	}()

	err = ftpConn.Stor(remoteFile, pr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Tar archive uploaded successfully.")
}
