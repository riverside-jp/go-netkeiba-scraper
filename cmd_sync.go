package main

import (
	"database/sql"
	"log"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

func cmdSync(c *cli.Context) error {
	db, err := util.openDatabase(filepath.Join(config.Path.DataDir, filenameDatabase))
	if err != nil {
		return xerrors.Errorf("Failed to open database: %+w", err)
	}
	defer db.Close()

	var dateRaw string

	if err := db.QueryRow("SELECT date FROM race ORDER BY date DESC LIMIT 1;").Scan(&dateRaw); err != nil && !xerrors.Is(err, sql.ErrNoRows) {
		return xerrors.Errorf("Failed to query row: %+w", err)
	}

	if dateRaw == "" {
		return xerrors.New("Nothing to synchronize: you may need to run `import` command")
	}

	date := strings.ReplaceAll(dateRaw, "-", "")

	raceTopURL := config.Netkeiba.DatabaseURL

	var racePages []string
L:
	for {
		time.Sleep(time.Second)

		schedulePages, prevMonthPage, err := findRaceSchedulePages(raceTopURL)
		if err != nil {
			log.Printf("Failed to send request to %s: %s", raceTopURL, err)
			continue
		}

		// sort schedulePages by date desc
		for i, j := 0, len(schedulePages)-1; i < j; i, j = i+1, j-1 {
			schedulePages[i], schedulePages[j] = schedulePages[j], schedulePages[i]
		}

		for i := 0; i < len(schedulePages); i++ {
			time.Sleep(time.Second)

			if strings.Contains(schedulePages[i], date) {
				break L
			}

			p, err := findRacePagesOfOneDay(config.Netkeiba.DatabaseURL, schedulePages[i])
			if err != nil {
				log.Printf("Failed to send request to %s: %s", schedulePages[i], err)
				continue
			}

			racePages = append(racePages, p...)
		}

		raceTopURL = config.Netkeiba.DatabaseURL + prevMonthPage
	}

	if len(racePages) < 1 {
		log.Println("It is up to date")
		return nil
	}

	if err := loginToNetkeibaCom(config.Netkeiba.LoginURL, config.Netkeiba.Email, config.Netkeiba.Password); err != nil {
		return err
	}

	for i := 0; i < len(racePages); i++ {
		if err := dumpWebPageAsHTMLFile(config.Path.DataDir, racePages[i]); err != nil {
			log.Printf("Failed to dump %s: %+v", racePages[i], err)
			continue
		}

		filename := filepath.Join(config.Path.DataDir, determineDumpHTMLFilenameFromURL(racePages[i]))

		if err := importRaceData(db, filename); err != nil {
			log.Printf("Failed to import %s: %s\n", filename, err)
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}
