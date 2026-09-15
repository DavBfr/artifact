package main

import (
	"crypto/rand"
	"errors"
	"net/http"

	"github.com/dgraph-io/badger/v4"
	"github.com/gorilla/mux"
)

const (
	shortSlugCharset        = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	shortSlugMinLen         = 5
	shortSlugMaxLen         = 12
	shortSlugAttemptsPerLen = 10
)

// randomSlug generates a random slug using crypto/rand: slugs gate access to
// uploaded files, so they must not be predictable.
func randomSlug(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	slug := make([]byte, length)
	for i, b := range buf {
		slug[i] = shortSlugCharset[int(b)%len(shortSlugCharset)]
	}
	return string(slug), nil
}

// generateUniqueSlug finds a slug not already present in txn, growing the
// length if repeated collisions occur.
func generateUniqueSlug(txn *badger.Txn) (string, error) {
	for length := shortSlugMinLen; length <= shortSlugMaxLen; length++ {
		for attempt := 0; attempt < shortSlugAttemptsPerLen; attempt++ {
			slug, err := randomSlug(length)
			if err != nil {
				return "", err
			}
			_, err = txn.Get([]byte("s:" + slug))
			if errors.Is(err, badger.ErrKeyNotFound) {
				return slug, nil
			}
			if err != nil {
				return "", err
			}
		}
	}
	return "", errors.New("failed to generate a unique short link slug")
}

// getOrCreateShortLink returns the existing slug for filename, creating one if needed.
func getOrCreateShortLink(filename string) (string, error) {
	var slug string
	err := shortLinkDB.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("f:" + filename))
		if err == nil {
			return item.Value(func(val []byte) error {
				slug = string(val)
				return nil
			})
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		slug, err = generateUniqueSlug(txn)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte("s:"+slug), []byte(filename)); err != nil {
			return err
		}
		return txn.Set([]byte("f:"+filename), []byte(slug))
	})
	if err != nil {
		return "", err
	}
	return slug, nil
}

// resolveShortLink returns the filename mapped to slug.
func resolveShortLink(slug string) (string, error) {
	var filename string
	err := shortLinkDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("s:" + slug))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			filename = string(val)
			return nil
		})
	})
	if err != nil {
		return "", err
	}
	return filename, nil
}

func shortLinkHandler(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]

	filename, err := resolveShortLink(slug)
	if errors.Is(err, badger.ErrKeyNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to resolve short link", http.StatusInternalServerError)
		return
	}

	serveFileByName(w, r, filename)
}
