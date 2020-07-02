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
		log.Printf("unsupported method: %s", req.Method)
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		log.Println("missing token")
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(tokenValue)
	if err != nil {
		log.Printf("invalid token: %v", err)
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/download/"))
	moduleName = cleanModuleName(moduleName)

	_, err = h.db.GetByName(moduleName)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Printf("unknown plugin: %s@%s", moduleName, version)
			jsonErrorf(rw, http.StatusNotFound, "unknown plugin: %s@%s", moduleName, version)
			return
		}

		log.Printf("failed to get plugin: %v", err)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
		return
	}

	sum := req.Header.Get(hashHeader)
	if sum != "" {
		ph, errH := h.db.GetHashByName(moduleName, version)
		if errH != nil {
			var notFoundError faunadb.NotFound
			if !errors.As(errH, &notFoundError) {
				log.Printf("failed to get plugin hash: %s@%s", moduleName, version)
				jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
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
		log.Printf("failed to download sources: %v", err)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
		return
	}

	defer func() { _ = sources.Close() }()

	_, err = h.db.GetHashByName(moduleName, version)
	var notFoundError faunadb.NotFound
	if !errors.As(err, &notFoundError) {
		log.Printf("failed to get plugin hash: %s@%s", moduleName, version)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
		return
	}

	if err == nil {
		_, err = io.Copy(rw, sources)
		if err != nil {
			log.Printf("failed to write response body (%s@%s): %v", moduleName, version, err)
			jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
			return
		}
		return
	}

	raw, err := ioutil.ReadAll(sources)
	if err != nil {
		log.Printf("failed to read response body (%s@%s): %v", moduleName, version, err)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
		return
	}

	hash := sha256.New()

	_, err = hash.Write(raw)
	if err != nil {
		log.Printf("failed to compute hash (%s@%s): %v", moduleName, version, err)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
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
		log.Printf("failed to write response body (%s@%s): %v", moduleName, version, err)
		jsonError(rw, http.StatusInternalServerError, "failed to get plugin")
		return
	}
}

// Validate validates a plugin archive.
func (h Handlers) Validate(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		log.Printf("unsupported method: %s", req.Method)
		jsonErrorf(rw, http.StatusMethodNotAllowed, "unsupported method: %s", req.Method)
		return
	}

	tokenValue := req.Header.Get(tokenHeader)
	if tokenValue == "" {
		log.Println("missing token")
		jsonError(rw, http.StatusBadRequest, "missing token")
		return
	}

	_, err := h.token.Check(tokenValue)
	if err != nil {
		log.Printf("invalid token: %v", err)
		jsonError(rw, http.StatusBadRequest, "invalid token")
		return
	}

	moduleName, version := path.Split(strings.TrimPrefix(req.URL.Path, "/validate/"))
	moduleName = strings.TrimSuffix(strings.TrimPrefix(moduleName, "/"), "/")

	headerSum := req.Header.Get(hashHeader)
	ph, err := h.db.GetHashByName(moduleName, version)
	if err != nil {
		var notFoundError faunadb.NotFound
		if errors.As(err, &notFoundError) {
			log.Printf("plugin not found: %s@%s", moduleName, version)
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
