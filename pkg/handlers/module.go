package handlers

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/fauna/faunadb-go/faunadb"
)

const (
	tokenHeader = "X-Token"
	hashHeader  = "X-Plugin-Hash"
)

// Download a plugin archive.
func (h Handlers) Download(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(tokenValue)
	if err != nil {
		log.Printf("invalid token: %v", err)
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/public/download/"))
	moduleName = cleanModuleName(moduleName)

	_, err = h.db.Get(moduleName)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			jsonErrorf(rw, http.StatusNotFound, "unknown plugin: %s@%s", moduleName, version)
			return
		}

		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	sum := req.Header.Get(hashHeader)
	if sum != "" {
		ph, errH := h.db.GetHashByName(moduleName, version)
		if errH != nil {
			var notFoundError faunadb.NotFound
			if !errors.As(errH, &notFoundError) {
				jsonError(rw, http.StatusInternalServerError, errH.Error())
				return
			}
		} else if ph.Hash == sum {
			rw.WriteHeader(http.StatusNotModified)
			return
		}

		log.Println("Someone is trying to hack the archive:", moduleName, version, sum)
	}

	sources, err := h.goProxy.DownloadSources(moduleName, version)
	if err != nil {
		jsonError(rw, http.StatusBadRequest, err.Error())
		return
	}

	defer func() { _ = sources.Close() }()

	_, err = h.db.GetHashByName(moduleName, version)
	var notFoundError faunadb.NotFound
	if !errors.As(err, &notFoundError) {
		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	if err == nil {
		_, err = io.Copy(rw, sources)
		if err != nil {
			jsonErrorf(rw, http.StatusInternalServerError, "failed to read response body: %v", err)
			return
		}
		return
	}

	raw, err := ioutil.ReadAll(sources)
	if err != nil {
		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	hash := sha256.New()

	_, err = hash.Write(raw)
	if err != nil {
		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	sum = fmt.Sprintf("%x", hash.Sum(nil))

	_, err = h.db.CreateHash(moduleName, version, sum)
	if err != nil {
		log.Printf("Error persisting plugin hash %s@%s: %v", moduleName, version, err)
		jsonError(rw, http.StatusInternalServerError, "could not persist data")
		return
	}

	_, err = rw.Write(raw)
	if err != nil {
		jsonErrorf(rw, http.StatusInternalServerError, "failed to write response body: %v", err)
		return
	}
}

// Validate validates a plugin archive.
func (h Handlers) Validate(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(tokenValue)
	if err != nil {
		log.Printf("invalid token: %v", err)
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/public/validate/"))
	moduleName = strings.TrimSuffix(strings.TrimPrefix(moduleName, "/"), "/")

	headerSum := req.Header.Get(hashHeader)
	ph, err := h.db.GetHashByName(moduleName, version)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			jsonError(rw, http.StatusNotFound, "plugin not found")
			return
		}

		jsonError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	if ph.Hash == headerSum {
		rw.WriteHeader(http.StatusOK)
		return
	}

	rw.WriteHeader(http.StatusNotFound)
}

func cleanModuleName(moduleName string) string {
	return strings.TrimSuffix(strings.TrimPrefix(moduleName, "/"), "/")
}
