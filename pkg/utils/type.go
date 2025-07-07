package utils


type Repo struct {
    Index       int    `json:"index"`
    Name        string `json:"name"`
    Url         string `json:"url"`
    Description string `json:"description"`
    Language    string `json:"language"`
    Stars       string `json:"stars"`
    Forks       string `json:"forks"`
}
