package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

func cmdImport(c *cli.Context) error {
	force := c.Bool("force")

	dbFilePath := filepath.Join(config.Path.DataDir, filenameDatabase)

	if err := setupDatabase(dbFilePath, force); err != nil {
		return xerrors.Errorf("Failed to setup database: %+w", err)
	}

	db, err := openDatabase(dbFilePath)
	if err != nil {
		return xerrors.Errorf("Failed to open database: %+w", err)
	}
	defer db.Close()

	files, err := filepath.Glob(filepath.Join(config.Path.DataDir, "*.html"))
	if err != nil {
		return xerrors.Errorf("Failed to glob HTML files: %+v", err)
	}

	log.Printf("Importing %d data ...\n", len(files))

	for i := 0; i < len(files); i++ {
		if err := importData(db, files[i]); err != nil {
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
		return nil
	}

	done := false

	db, err := openDatabase(dbFilePath)
	if err != nil {
		return xerrors.Errorf("Failed to open database: %+w", err)
	}

	defer func() {
		db.Close()
		if !done {
			os.Remove(dbFilePath)
		}
	}()

	query := `
	CREATE TABLE IF NOT EXISTS race_info (
		id            INTEGER PRIMARY KEY,
		name          TEXT    NOT NULL,
		racetrack     TEXT    NOT NULL,
		race_number   INTEGER NOT NULL,
		surface       TEXT    NOT NULL,
		course        TEXT    NOT NULL,
		distance      INTEGER NOT NULL,
		weather       TEXT    NOT NULL,
		surface_state TEXT    NOT NULL,
		race_start    TEXT    NOT NULL,
		surface_score INTEGER,
		date          TEXT    NOT NULL,
		place_detail  TEXT    NOT NULL,
		class         INTEGER NOT NULL CHECK(class between -1 and 7),
		class_detail  TEXT    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS date_idx    ON race_info(date);
	CREATE INDEX IF NOT EXISTS id_date_idx ON race_info(id, date);

	CREATE TABLE IF NOT EXISTS race_result (
		race_id            INTEGER  NOT NULL,
		order_of_finish    TEXT     NOT NULL,
		frame_number       INTEGER  NOT NULL,
		horse_number       INTEGER  NOT NULL,
		horse_id           TEXT     NOT NULL,
		horse_name         TEXT     NOT NULL,
		sex                TEXT     NOT NULL,
		age                INTEGER  NOT NULL,
		basis_weight       REAL     NOT NULL,
		jockey_id          TEXT     NOT NULL,
		jockey_name        TEXT     NOT NULL,
		finishing_time     TEXT     NOT NULL,
		length             TEXT     NOT NULL,
		speed_figure       INTEGER,
		pass               TEXT     NOT NULL,
		last_phase         REAL,
		odds               REAL,
		popularity         INTEGER,
		horse_weight       TEXT     NOT NULL,
		remark             TEXT,
		stable             TEXT     NOT NULL,
		trainer_id         TEXT     NOT NULL,
		owner_id           TEXT     NOT NULL,
		earning_money      REAL,
		PRIMARY KEY (race_id, horse_number),
		FOREIGN KEY (race_id) REFERENCES race_info(id)
	);
	CREATE INDEX IF NOT EXISTS race_id_idx               ON race_result (race_id);
	CREATE INDEX IF NOT EXISTS race_id_horse_id_idx      ON race_result (race_id, horse_id);
	CREATE INDEX IF NOT EXISTS race_id_jockey_id_idx     ON race_result (race_id, jockey_id);
	CREATE INDEX IF NOT EXISTS race_id_trainer_id_idx    ON race_result (race_id, trainer_id);
	CREATE INDEX IF NOT EXISTS race_id_owner_id_idx      ON race_result (race_id, owner_id);
	CREATE INDEX IF NOT EXISTS horse_id_speed_figure_idx ON race_result (horse_id, speed_figure);
	CREATE INDEX IF NOT EXISTS trainer_id_idx            ON race_result (trainer_id);
	CREATE INDEX IF NOT EXISTS owner_id_idx              ON race_result (owner_id);
	CREATE INDEX IF NOT EXISTS jockey_id_idx             ON race_result (jockey_id);

	CREATE TABLE IF NOT EXISTS payoff (
		race_id      INTEGER NOT NULL,
		ticket_type  INTEGER NOT NULL CHECK(ticket_type between -1 and 7),
		horse_number TEXT    NOT NULL,
		payoff       REAL    NOT NULL CHECK(payoff >= 0),
		popularity   INTEGER NOT NULL CHECK(popularity >= 0),
		PRIMARY KEY (race_id, ticket_type, horse_number),
		FOREIGN KEY (race_id) REFERENCES race_info(id)
	);
	`

	if _, err := db.Exec(query); err != nil {
		return xerrors.Errorf("Failed to execute query(%s): %+w", query, err)
	}

	done = true

	return nil
}

func openDatabase(dbFilePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func importData(db *sql.DB, filePath string) error {
	id, _ := strconv.Atoi(strings.TrimSuffix(filepath.Base(filePath), ".html"))

	doc, err := htmlquery.LoadDoc(filePath)
	if err != nil {
		return xerrors.Errorf("Failed to read file %s: %+w", filePath, err)
	}

	r1, err := buildRaceInformationRecord(id, doc)
	if err != nil {
		return xerrors.Errorf("Failed to build race information record: %+w", err)
	}

	r2, err := buildPayoffRecords(id, doc)
	if err != nil {
		return xerrors.Errorf("Failed to build payoff record: %+w", err)
	}

	r3, err := buildRaceResultRecords(id, doc)
	if err != nil {
		return xerrors.Errorf("Failed to build payoff record: %+w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return xerrors.Errorf("Failed to begin transaction: %+w", err)
	}

	s1, err := tx.Prepare(`
	INSERT OR REPLACE INTO race_info (
		id,
		name,
		racetrack,
		race_number,
		surface,
		course,
		distance,
		weather,
		surface_state,
		race_start,
		surface_score,
		date,
		place_detail,
		class,
		class_detail
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return xerrors.Errorf("Failed to prepare statement: %+w", err)
	}
	defer s1.Close()

	if _, err := s1.Exec(
		r1.id,
		r1.name,
		r1.racetrack,
		r1.raceNumber,
		r1.surface,
		r1.course,
		r1.distance,
		r1.weather,
		r1.surfaceState,
		r1.raceStart,
		r1.surfaceScore,
		r1.date,
		r1.placeDetail,
		r1.class,
		r1.classDetail,
	); err != nil {
		return xerrors.Errorf("Failed to executes a prepared statement: %+w", err)
	}

	s2, err := tx.Prepare(`
	INSERT OR REPLACE INTO payoff (
		race_id,
		ticket_type,
		horse_number,
		payoff,
		popularity
	) VALUES (?, ?, ?, ?, ?);
	`)
	if err != nil {
		return xerrors.Errorf("Failed to prepare statement: %+w", err)
	}
	defer s2.Close()

	for i := 0; i < len(r2); i++ {
		if _, err := s2.Exec(
			r2[i].raceID,
			r2[i].ticketType,
			r2[i].horseNumber,
			r2[i].payoff,
			r2[i].popularity,
		); err != nil {
			return xerrors.Errorf("Failed to execute a prepared statement: %+w", err)
		}
	}

	s3, err := tx.Prepare(`
	INSERT OR REPLACE INTO race_result (
		race_id,
		order_of_finish,
		frame_number,
		horse_number,
		horse_id,
		horse_name,
		sex,
		age,
		basis_weight,
		jockey_id,
		jockey_name,
		finishing_time,
		length,
		speed_figure,
		pass,
		last_phase,
		odds,
		popularity,
		horse_weight,
		remark,
		stable,
		trainer_id,
		owner_id,
		earning_money
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return xerrors.Errorf("Failed to prepare statement: %+w", err)
	}
	defer s3.Close()

	for i := 0; i < len(r3); i++ {
		if _, err := s3.Exec(
			r3[i].raceID,
			r3[i].orderOfFinish,
			r3[i].frameNumber,
			r3[i].horseNumber,
			r3[i].horseID,
			r3[i].horseName,
			r3[i].sex,
			r3[i].age,
			r3[i].basisWeight,
			r3[i].jockeyID,
			r3[i].jockeyName,
			r3[i].finishingTime,
			r3[i].length,
			r3[i].speedFigure,
			r3[i].pass,
			r3[i].lastPhase,
			r3[i].odds,
			r3[i].popularity,
			r3[i].horseWeight,
			r3[i].remark,
			r3[i].stable,
			r3[i].trainerID,
			r3[i].ownerID,
			r3[i].earningMoney,
		); err != nil {
			return xerrors.Errorf("Failed to executes a prepared statement: %+w", err)
		}
	}

	return tx.Commit()
}
