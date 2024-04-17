package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

const baseIssueURL = "https://www.jair.org/index.php/jair/issue/view/"

var log = logrus.New()

func setupLogger() {
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})
	log.SetLevel(logrus.InfoLevel)
}

func generateIssueURL(issueID int) string {
	return fmt.Sprintf("%s%d", baseIssueURL, issueID)
}

func findPDFViewerLinks(issueURL string) ([]string, []string, error) {
	resp, err := http.Get(issueURL)
	if err != nil {
		log.WithField("url", issueURL).Error("Failed to fetch issue page:", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.WithField("url", issueURL).Error("Failed to parse HTML document:", err)
		return nil, nil, err
	}

	var links []string
	var titles []string
	doc.Find(".article-summary .media-body").Each(func(i int, s *goquery.Selection) {
		title := s.Find("h3.media-heading a").Text()
		viewerLink, exists := s.Find(".btn-group a.pdf").Attr("href")
		if exists {
			fullLink := resolveURL(viewerLink)
			links = append(links, fullLink)
			titles = append(titles, strings.TrimSpace(title))
		}
	})

	return links, titles, nil
}

func resolveURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return "https://www.jair.org" + href
}

func extractActualPDFLink(pageURL string) (string, error) {
	resp, err := http.Get(pageURL)
	if err != nil {
		log.WithField("url", pageURL).Error("Failed to fetch viewer page:", err)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithField("url", pageURL).Error("Failed to read response body:", err)
		return "", err
	}
	bodyString := string(bodyBytes)

	re := regexp.MustCompile(`var pdfUrl = "([^"]+)"`)
	matches := re.FindStringSubmatch(bodyString)
	if len(matches) < 2 {
		log.WithField("url", pageURL).Error("No PDF URL found in script")
		return "", fmt.Errorf("no PDF URL found in script")
	}

	decodedURL, err := url.QueryUnescape(matches[1])
	if err != nil {
		log.WithField("encoded URL", matches[1]).Error("Failed to decode URL:", err)
		return "", err
	}

	decodedURL = strings.ReplaceAll(decodedURL, `\/`, `/`)
	return decodedURL, nil
}

func downloadPDF(pdfURL, filePath string) error {
	resp, err := http.Get(pdfURL)
	if err != nil {
		log.WithField("PDF URL", pdfURL).Error("Failed to download PDF:", err)
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		log.WithField("file path", filePath).Error("Failed to create file:", err)
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.WithField("file path", filePath).Error("Failed to write PDF:", err)
	}
	log.WithField("file path", filePath).Info("PDF downloaded successfully")
	return err
}

func main() {
	setupLogger()
	outputDir := flag.String("output", ".", "Directory to save downloaded PDFs")
	endIssueCount := flag.Int("endIssue", 75, "The number of issues to include from 1, starting id is 1085")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Error("Failed to create output directory:", err)
		return
	}

	finalIssueID := 1085 + *endIssueCount - 1

	for issueID := 1085; issueID <= finalIssueID; issueID++ {
		issueURL := generateIssueURL(issueID)
		log.Info("Processing issue:", issueURL)
		viewerLinks, titles, err := findPDFViewerLinks(issueURL)
		if err != nil {
			log.WithField("issue", issueID).Error("Failed to find viewer links:", err)
			continue
		}

		for j, link := range viewerLinks {
			title := titles[j]
			log.Info("Found PDF viewer link:", link)
			actualPDFLink, err := extractActualPDFLink(link)
			if err != nil {
				log.WithField("title", title).Error("Failed to extract PDF link from viewer:", err)
				continue
			}

			fileName := fmt.Sprintf("%s.pdf", title)
			filePath := filepath.Join(*outputDir, fileName)
			err = downloadPDF(actualPDFLink, filePath)
			if err != nil {
				log.WithField("title", title).Error("Failed to download PDF:", err)
				continue
			}
			log.WithField("title", title).Info("Successfully downloaded:", filePath)
		}
	}
}
