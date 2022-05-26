package main

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"github.com/boofexxx/errors"
)

const (
	MB = 1024 * 1024
)

func (mux *Server) saveFile(fileIn multipart.File, path string) error {
	op := errors.Op("Server.saveFile")
	log.Printf("creating %s\n", path)
	err := os.Mkdir(path, os.ModePerm)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return errors.E(err, op)
		}
		log.Printf("%s already exists. removing", path)
		if err = os.RemoveAll(path); err != nil {
			return errors.E(err, op)
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
			return errors.E(err, op)
		}
		filePath := path + string(os.PathSeparator) + strconv.Itoa(i)
		log.Printf("creating %s\n", filePath)
		fileOut, err := os.Create(filePath)
		if err != nil {
			return errors.E(err, op)
		}
		if _, err = fileOut.Write(buff); err != nil {
			return errors.E(err, op)
		}
		fileOut.Close()
		buff = make([]byte, MB)
	}
	return nil
}

func (mux *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("Server.uploadHandler")
	if err := r.ParseMultipartForm(64 * MB); err != nil {
		err = errors.E(op, err, "parse multipart form")
		http.Error(w, errors.Innermost(err).Error(), http.StatusBadRequest)
		log.Print(err)
		return
	}
	f, fh, err := r.FormFile("file")
	if err != nil {
		err = errors.E(op, err, "form file")
		http.Error(w, errors.Innermost(err).Error(), http.StatusBadRequest)
		log.Print(err)
		return
	}
	path := mux.dirName + string(os.PathSeparator) + fh.Filename
	if err = mux.saveFile(f, path); err != nil {
		err = errors.E(err, op)
		http.Error(w, errors.Innermost(err).Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	w.Write([]byte(path + " saved"))
}

func (mux *Server) collectFiles(path string) ([]byte, error) {
	op := errors.Op("Server.collectFiles")
	fs := os.DirFS(path)
	// probably we could count number of files and preallocate expected number of bytes
	out := make([]byte, 0)

	for i := 0; ; i++ {
		buf := make([]byte, MB)
		f, err := fs.Open(strconv.Itoa(i))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if len(out) == 0 {
					return nil, errors.E(err, op)
				}
				return bytes.TrimRight(out, "\x00"), nil
			}
			return nil, errors.E(err, op)
		}

		_, err = f.Read(buf)
		if err != nil {
			return nil, err
		}
		out = append(out, buf...)
	}
}

func (mux *Server) downloadHandler(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("Server.downloadHandler")
	path := mux.dirName + string(os.PathSeparator) + r.FormValue("file")
	buff, err := mux.collectFiles(path)
	if err != nil {
		err = errors.E(op, err)
		http.Error(w, errors.Innermost(err).Error(), http.StatusInternalServerError)
		log.Print(err)
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
		log.Fatalf("couldn't create directory %s: %v", mux.dirName, err)
	}

	mux.HandleFunc("/upload", mux.uploadHandler)
	mux.HandleFunc("/download", mux.downloadHandler)

	log.Println("started listening at :8080...")
	http.ListenAndServe(":8080", &mux)
}
