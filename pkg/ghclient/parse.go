package ghclient

import (
	"strings"
	"ghtrend/pkg/types"

	"github.com/PuerkitoBio/goquery"
)


func parseRootInfo(html string) ([]types.EntryInfor, error) {
	r := strings.NewReader(html)
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var entries []types.EntryInfor
	doc.Find("a.Link--primary").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		name := strings.TrimSpace(s.Text())
		t := "unknown"
		switch {
		case strings.Contains(href, "/tree/"):
			t = "dir"
		case strings.Contains(href, "/blob/"):
			t = "file"
		default:
			if aria, ok := s.Attr("aria-label"); ok {
				if strings.Contains(aria, "Directory") {
					t = "dir"
				} else if strings.Contains(aria, "File") {
					t = "file"
				}
			}
		}
		entries = append(entries, types.EntryInfor{ 
			Name: name, 
			Type: t,
		})
	})
	return entries, nil
}
