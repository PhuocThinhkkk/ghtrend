package httpRequest

import (
	"io"
	"net/http"
)

func Fetch(url string) ([]byte, error) {
	res, err := http.Get(url)
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

// https://raw.githubusercontent.com/charmbracelet/glow/main/README.md
func GetRawGithubReadmeFile( owner string, repoName string ) ( string , error ) {
	url := "https://raw.githubusercontent.com/" + owner + "/" + repoName + "/master/README.md"
	readmeText, err := Fetch(url)
	if err == nil {
		return string(readmeText), nil
	}

	url2 := "https://raw.githubusercontent.com/" + owner + "/" + repoName + "/master/README.md"
	readmeText2, err := Fetch(url2)
	if err != nil {
		return "", err
	}
	return string(readmeText2), nil
}




