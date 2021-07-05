package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

func cmdImport(c *cli.Context) error {
	force := c.Bool("force")

	dbFilePath := filepath.Join(config.Path.DataDir, filenameDatabase)

	if err := setupDatabase(dbFilePath, force); err != nil {
		return xerrors.Errorf("Failed to setup database: %+w", err)
	}

	db, err := util.openDatabase(dbFilePath)
	if err != nil {
		return xerrors.Errorf("Failed to open database: %+w", err)
	}
	defer db.Close()

	files, err := filepath.Glob(filepath.Join(config.Path.DataDir, "*.html"))
	if err != nil {
		return xerrors.Errorf("Failed to glob HTML files: %+w", err)
	}

	log.Printf("Importing %d race data ...\n", len(files))

	for i := 0; i < len(files); i++ {
		if err := importRaceData(db, files[i]); err != nil {
			log.Printf("Failed to import %s: %s\n", files[i], err)
		}
	}

	files, err = filepath.Glob(filepath.Join(config.Path.DataDir, "horse", "*.html"))
	if err != nil {
		return xerrors.Errorf("Failed to glob HTML files: %+w", err)
	}

	log.Printf("Importing %d horse data ...\n", len(files))

	for i := 0; i < len(files); i++ {
		if err := importHorseData(db, files[i]); err != nil {
			log.Printf("Failed to import %s: %s\n", files[i], err)
		}
	}

	log.Println("Succeeded to import data")

	return nil
}

func setupDatabase(dbFilePath string, force bool) error {
	if force {
		os.Remove(dbFilePath)
	}

	if _, err := os.Stat(dbFilePath); err == nil {
		return nil // already set up
	}

	done := false

	db, err := util.openDatabase(dbFilePath)
	if err != nil {
		return err
	}

	defer func() {
		db.Close()
		if !done {
			os.Remove(dbFilePath)
		}
	}()

	b, err := ioutil.ReadFile("./schema.sql")
	if err != nil {
		return err
	}

	if _, err := db.Exec(string(b)); err != nil {
		return err
	}

	done = true

	return nil
}
