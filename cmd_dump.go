package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

func cmdDump(c *cli.Context) error {
	dataType := c.String("data-type")

	if dataType == "horse" {
		return dumpHorseData()
	}

	return dumpRaceData()
}

func dumpRaceData() error {
	file, err := os.Open(filepath.Join(config.Path.DataDir, filenameRaceList))
	if err != nil {
		return xerrors.Errorf("Failed to open file: %+w", err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return xerrors.Errorf("Failed to read file: %+w", err)
	}

	if err := loginToNetkeibaCom(config.Netkeiba.LoginURL, config.Netkeiba.Email, config.Netkeiba.Password); err != nil {
		return xerrors.Errorf("Failed to login netkeiba.com: %+w", err)
	}

	racePages := strings.Split(string(b), "\n")

	for i := 0; i < len(racePages); i++ {
		if err := dumpWebPageAsHTMLFile(config.Path.DataDir, racePages[i]); err != nil {
			log.Printf("Failed to dump %s: %+v", racePages[i], err)
			continue
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

func dumpHorseData() error {
	dbFilePath := filepath.Join(config.Path.DataDir, filenameDatabase)

	db, err := util.openDatabase(dbFilePath)
	if err != nil {
		return xerrors.Errorf("Failed to open database: %+w", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT DISTINCT(horse_id) FROM result ORDER BY horse_id ASC")
	if err != nil {
		return err
	}

	path := filepath.Join(config.Path.DataDir, "horse")

	for rows.Next() {
		var horseID string

		if err := rows.Scan(&horseID); err != nil {
			return err
		}

		url := config.Netkeiba.DatabaseURL + "/horse/ped/" + horseID

		if err := dumpWebPageAsHTMLFile(path, url); err != nil {
			log.Printf("Failed to dump %s: %+v", url, err)
			continue
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}
