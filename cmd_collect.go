package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
