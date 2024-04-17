package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const baseIssueID = 1085

func generateIssueURL(issueID int) string {
	return fmt.Sprintf("https://www.jair.org/index.php/jair/issue/view/%d", issueID)
}

func findPDFLinks(issueURL string) ([]string, []string, error) {
	resp, err := http.Get(issueURL)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var links []string
	var titles []string
	doc.Find(".article-summary .media-body").Each(func(i int, s *goquery.Selection) {
		title := s.Find("h3.media-heading a").Text()
		pdfLink, exists := s.Find(".btn-group a.pdf").Attr("href")
		if exists {
			links = append(links, pdfLink)
			titles = append(titles, strings.TrimSpace(title))
		}
	})

	return links, titles, nil
}

func downloadPDF(pdfURL, filePath string) error {
	resp, err := http.Get(pdfURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func main() {
	outputDir := flag.String("output", ".", "Directory to save downloaded PDFs")
	//Ids on the website begin at 1085 for issue 1.
	endIssueCount := flag.Int("endIssue", 75, "The number of issues to include.")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Println("Failed to create output directory:", err)
		return
	}

	finalIssueID := baseIssueID + *endIssueCount - 1

	for issueID := baseIssueID; issueID <= finalIssueID; issueID++ {
		issueURL := generateIssueURL(issueID)
		fmt.Println("Processing:", issueURL)
		pdfLinks, titles, err := findPDFLinks(issueURL)
		if err != nil {
			fmt.Println("Failed to find PDF links for issue", issueID, ":", err)
			continue
		}

		for j, link := range pdfLinks {
			title := titles[j]
			fmt.Println("Downloading PDF from:", link, "Title:", title)
			safeTitle := strings.ReplaceAll(title, " ", "_")
			fileName := fmt.Sprintf("%s.pdf", safeTitle)
			filePath := filepath.Join(*outputDir, fileName)
			err = downloadPDF(link, filePath)
			if err != nil {
				fmt.Println("Failed to download PDF from", link, ":", err)
				continue
			}
			fmt.Println("Successfully downloaded:", filePath)
		}
	}
}
