package main

import (
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

//blackBoardMsg contains a parsed OC-Campus Message
type blackBoardMsg struct {
	Title   string
	Date    string
	Message string
	Link    string
}

func parsePrivateMessagesSection(data string) {

	// Create HTML Document
	output := html.UnescapeString(data)
	html := replaceUmlauts(output)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))

	if err != nil {
		log.Println("Couldn't parse html of Notifications")
		return
	}
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		dateMatch := true
		s.Find("td").Each(func(h int, row *goquery.Selection) {
			if h == 2 { // Check for todays message
				day := time.Now().Format("01.02.2006")
				if strings.Contains(row.Text(), day) {
					dateMatch = true
				}
			}
			if dateMatch { // Parse todays message for notifications
				if h == 5 {
					if strings.Contains(row.Text(), "Ihre Videokonferenz startet in Kuerze um") {
						subject := row.Text()
						msgLink, _ := row.Find("a").Attr("href")
						fmt.Println("Course: ", msgLink)
						parseNotification(subject, msgLink)
					}
				}
			}
		})
	})

}

func parseNotification(subject, notfiyLink string) {
	url := endpoint + notfiyLink

	// Prepare new HTTP request
	request, err := http.NewRequest("GET", url, nil)
	request.Header.Add("Content-Type", "charset=UTF-8")

	// Send HTTP request and move the response to the variable
	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode == 200 {
		data, errB := ioutil.ReadAll(response.Body)
		if errB != nil {
			log.Println("Error decoding Body of Notifications")
		}
		// Create HTML Document
		output := html.UnescapeString(string(data))
		html := replaceUmlauts(output)
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		s := doc.Find("table").Find("fieldset")

		s.Contents().Each(func(i int, s *goquery.Selection) {
			if !s.Is("br") {
				r, _ := regexp.Compile("^http|https+://*$") // This matches a line that contains only a link
				if r.Match([]byte(s.Text())) {
					fmt.Println("Link:", s.Text())
					sendCourseNotification(s.Text(), subject[1:])
				}
			}
		})

	}
}

func parseBlackBoardData(d blackboardRes) {
	if d.Status == 200 {
		output := html.UnescapeString(d.HTML)
		html := replaceUmlauts(output)

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))

		if err != nil {
			log.Println("Couldn't parse html of BlackBoardData")
			return
		}
		// Find the news  items
		doc.Find("#cell_blackboardtype1").Each(func(i int, s *goquery.Selection) {
			// For each item found, parse the message
			s.Find("li").Each(parseMessageHTML)
		})
		// Find the news  items
		doc.Find("#cell_mPrio").Each(func(i int, s *goquery.Selection) {
			// For each item found, parse the message
			s.Find("li").Each(parseMessageHTML)
		})
		return // skip
	}
	log.Println("Detected error message in blackBoardRes while trying to parse. Seems like an API Error. aborting.. ")
}

func parseMessageHTML(i int, s *goquery.Selection) {
	// Only parse msgs with content in it
	if !s.Is(":empty") {
		log.Println("Got new Data in Blackboard. Starting parsing process..... ")
		// Find all Values in HTML Doc
		title := s.Find(".titel").Text()
		date := s.Find(".date").Text()
		body := s.Find(".abstract").Text()
		link, state := s.Find(".abstract").Find("a").Attr("href")
		if !state {
			log.Println("Message", title, "does not contain an Hyperlink for more information")
		}

		// Cleanup and create message object
		body = replaceUmlauts(body)
		richBody := parseMessageBodyFromRef(link)
		if richBody != "" {
			body = richBody
		}

		msg := blackBoardMsg{
			Title:   title,
			Date:    date,
			Message: body,
			Link:    link,
		}
		msgQueue = append(msgQueue, msg) // Add Item to queue to be parsed
		return
	}
	log.Print("Couldn't find any new articles. \n")
}

func parseMessageBodyFromRef(ref string) string {
	var msgString string

	url := endpoint + ref
	// Prepare new HTTP request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Add("Content-Type", "charset=UTF-8")

	// Send HTTP request and move the response to the variable
	res, err := client.Do(request)
	if err != nil {
		log.Println("Cant get document from link, seems invalid", err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		doc, _ := goquery.NewDocumentFromReader(res.Body)
		// Parse each <p> Tag in the content div where the message is displayed.
		doc.Find("#content").Find("p").Each(func(i int, s *goquery.Selection) {
			if !s.Is(":empty") {
				txt := s.Text()
				if strings.Contains(txt, "Übersicht") { // Skip if its the "Übersicht" Dialog that is not relevant for the message
					return
				}
				msgString += txt + "\n"
			}
		})
	}

	// Cleanup first few D
	msgString = strings.Replace(msgString, "\n", "", 4)
	return msgString
}
