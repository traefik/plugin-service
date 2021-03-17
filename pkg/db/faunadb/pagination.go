package faunadb

import (
	"encoding/base64"
	"encoding/json"

	f "github.com/fauna/faunadb-go/v3/faunadb"
)

// NextPageList represents a pagination header value.
type NextPageList struct {
	Stars  int    `json:"stars"`
	NextID string `json:"nextId"`
}

// NextPageSearch represents a pagination header value for the search request.
type NextPageSearch struct {
	Name   string `json:"name"`
	NextID string `json:"nextId"`
}

func encodeNextPageList(after f.ArrayV) (string, error) {
	if len(after) <= 2 {
		return "", nil
	}

	var nextN int
	err := after[0].Get(&nextN)
	if err != nil {
		return "", err
	}

	var next f.RefV
	err = after[1].Get(&next)
	if err != nil {
		return "", err
	}

	nextPage := NextPageList{NextID: next.ID, Stars: nextN}

	b, err := json.Marshal(nextPage)
	if err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(b), nil
}

func decodeNextPageList(data string) (NextPageList, error) {
	decodeString, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return NextPageList{}, err
	}

	var nextPage NextPageList
	err = json.Unmarshal(decodeString, &nextPage)
	if err != nil {
		return NextPageList{}, err
	}
	return nextPage, nil
}

func encodeNextPageSearch(after f.ArrayV) (string, error) {
	if len(after) <= 2 {
		return "", nil
	}

	var nextN string
	err := after[0].Get(&nextN)
	if err != nil {
		return "", err
	}

	var next f.RefV
	err = after[1].Get(&next)
	if err != nil {
		return "", err
	}

	nextPage := NextPageSearch{NextID: next.ID, Name: nextN}

	b, err := json.Marshal(nextPage)
	if err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(b), nil
}

func decodeNextPageSearch(data string) (NextPageSearch, error) {
	decodeString, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return NextPageSearch{}, err
	}

	var nextPage NextPageSearch
	err = json.Unmarshal(decodeString, &nextPage)
	if err != nil {
		return NextPageSearch{}, err
	}
	return nextPage, nil
}
