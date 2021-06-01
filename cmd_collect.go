package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

func cmdCollect(c *cli.Context) error {
	raceTopURL := config.Netkeiba.DatabaseURL

	file, err := os.OpenFile(filepath.Join(config.Path.DataDir, filenameRaceList), os.O_WRONLY|os.O_CREATE, os.FileMode(0666))
	if err != nil {
		return xerrors.Errorf("Failed to open file: %+w", err)
	}
	defer file.Close()

	for i := 0; i < determineCollectOffset(c); i++ {
		time.Sleep(time.Second)

		schedulePages, prevMonthPage, err := findRaceSchedulePages(raceTopURL)
		if err != nil {
			log.Printf("Failed to send request to %s: %s", raceTopURL, err)
			continue
		}

		var racePages []string

		for i := 0; i < len(schedulePages); i++ {
			time.Sleep(time.Second)

			p, err := findRacePagesOfOneDay(config.Netkeiba.DatabaseURL, schedulePages[i])
			if err != nil {
				log.Printf("Failed to send request to %s: %s", schedulePages[i], err)
				continue
			}

			racePages = append(racePages, p...)
		}

		if 0 < len(racePages) {
			file.WriteString(strings.Join(racePages, "\n") + "\n")
		}

		raceTopURL = config.Netkeiba.DatabaseURL + prevMonthPage
	}

	return nil
}

func determineCollectOffset(c *cli.Context) int {
	i := c.Int("years")

	if 0 < i && i <= 10 {
		return i
	}

	return 10 * 12 // return 10 years * 12 months by default
}

func findRaceSchedulePages(url string) (pages []string, prevMonthPage string, err error) {
	c := colly.NewCollector()

	c.OnHTML("div.race_calendar table a", func(e *colly.HTMLElement) {
		pages = append(pages, e.Attr("href"))
	})

	c.OnHTML("div.race_calendar .rev a:last-child", func(e *colly.HTMLElement) {
		prevMonthPage = e.Attr("href")
	})

	c.OnRequest(func(request *colly.Request) {
		log.Println("Sending request to " + url)
	})

	if err := c.Visit(url + "/?pid=race_top"); err != nil {
		return nil, "", err
	}

	return pages, prevMonthPage, nil
}

func findRacePagesOfOneDay(baseURL string, path string) ([]string, error) {
	var races []string

	c := colly.NewCollector()

	c.OnHTML("dl.race_top_data_info dd > a", func(e *colly.HTMLElement) {
		races = append(races, baseURL + e.Attr("href"))
	})

	c.OnRequest(func(request *colly.Request) {
		log.Println("Sending request to " + baseURL + path)
	})

	if err := c.Visit(baseURL + path); err != nil {
		return nil, err
	}

	return races, nil
}
