package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
)

const (
	MB = 1024 * 1024
)

func (mux *Server) saveFile(fileIn multipart.File, path string) error {
	log.Printf("creating %s\n", path)
	err := os.Mkdir(path, os.ModePerm)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		log.Printf("%s already exists. removing", path)
		if err = os.RemoveAll(path); err != nil {
			return err
		}
		return mux.saveFile(fileIn, path)
	}

	buff := make([]byte, MB)
	for i := 0; ; i++ {
		_, err := fileIn.Read(buff)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		filePath := path + string(os.PathSeparator) + strconv.Itoa(i)
		log.Printf("creating %s\n", filePath)
		fileOut, err := os.Create(filePath)
		if err != nil {
			return err
		}
		if _, err = fileOut.Write(buff); err != nil {
			return err
		}
		fileOut.Close()
		buff = make([]byte, MB)
	}
	return nil
}

func (mux *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 * MB); err != nil {
		http.Error(w, fmt.Sprintf("expected multipart form: %v", err), http.StatusBadRequest)
		return
	}
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("couldn't form file: %v", err), http.StatusBadRequest)
		return
	}

	path := mux.dirName + string(os.PathSeparator) + fh.Filename
	if err = mux.saveFile(f, path); err != nil {
		http.Error(w, fmt.Sprintf("couldn't save file: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(path + " saved"))
}

func (mux *Server) collectFiles(path string) ([]byte, error) {
	fs := os.DirFS(path)
	// probably we could count number of files and preallocate expected number of bytes
	out := make([]byte, 0)

	for i := 0; ; i++ {
		buf := make([]byte, MB)
		f, err := fs.Open(strconv.Itoa(i))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if len(out) == 0 {
					return nil, err
				}
				return bytes.TrimRight(out, "\x00"), nil
			}
			return nil, err
		}

		_, err = f.Read(buf)
		if err != nil {
			return nil, err
		}
		out = append(out, buf...)
	}
}

func (mux *Server) downloadHandler(w http.ResponseWriter, r *http.Request) {
	path := mux.dirName + string(os.PathSeparator) + r.FormValue("file")
	buff, err := mux.collectFiles(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Couldn't collect files: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(buff)
}

type Server struct {
	http.ServeMux
	dirName string
}

func main() {
	mux := Server{dirName: "downloads"}

	err := os.Mkdir(mux.dirName, os.ModePerm)
	if err != nil && !errors.Is(err, os.ErrExist) {
		log.Fatalf("Couldn't create directory %s: %v", mux.dirName, err)
	}

	mux.HandleFunc("/upload", mux.uploadHandler)
	mux.HandleFunc("/download", mux.downloadHandler)

	log.Println("started listening at :8080...")
	http.ListenAndServe(":8080", &mux)
}
