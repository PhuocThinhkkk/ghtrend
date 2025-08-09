package types

type Repo struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	Url         string `json:"url"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       string `json:"stars"`
	Forks       string `json:"forks"`
	ReadMe      string `json:"readme"` 
	RootInfor   []EntryInfor  `json:"root_infor"`
	
}


type EntryInfor struct {
	Name   string `json:"name"`
	Type   string  `json:"type"`
}



type CacheData struct {
	Timestamp int64 `json:"timestamp"`
	Data     []Repo      `json:"data"`
}
