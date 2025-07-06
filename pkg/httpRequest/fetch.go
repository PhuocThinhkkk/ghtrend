package httpRequest

import (
	"io"
	"net/http"
)

func Fetch() ([]byte, error) {
	res, err := http.Get("https://github.com/trending")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

