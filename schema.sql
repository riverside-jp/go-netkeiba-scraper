CREATE TABLE IF NOT EXISTS `race` (
    id             INTEGER PRIMARY KEY,
    name           TEXT    NOT NULL,
    course         TEXT    NOT NULL,
    number         INTEGER NOT NULL,
    surface        TEXT    NOT NULL,
    direction      TEXT    NOT NULL,
    distance       INTEGER NOT NULL,
    weather        TEXT    NOT NULL,
    surface_state  TEXT    NOT NULL,
    surface_index  INTEGER,
    date           TEXT    NOT NULL,
    post_time      TEXT    NOT NULL,
    classification TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS date_idx    ON race(date);
CREATE INDEX IF NOT EXISTS id_date_idx ON race(id, date);

CREATE TABLE IF NOT EXISTS `result` (
    race_id            INTEGER  NOT NULL,
    order_of_finish    TEXT     NOT NULL,
    bracket            INTEGER  NOT NULL,
    draw               INTEGER  NOT NULL,
    horse_id           INTEGER  NOT NULL,
    horse              TEXT     NOT NULL,
    sex                TEXT     NOT NULL,
    age                INTEGER  NOT NULL,
    weight             REAL     NOT NULL,
    jockey_id          TEXT     NOT NULL,
    jockey             TEXT     NOT NULL,
    time               TEXT     NOT NULL,
    winning_margin     TEXT     NOT NULL,
    speed_index        INTEGER,
    position           TEXT     NOT NULL,
    sectional_time     REAL,
    odds               REAL,
    popularity         INTEGER,
    horse_weight       TEXT     NOT NULL,
    note               TEXT,
    stable             TEXT     NOT NULL,
    trainer_id         TEXT     NOT NULL,
    owner_id           TEXT     NOT NULL,
    earnings           REAL,
    PRIMARY KEY (race_id, horse_id),
    FOREIGN KEY (race_id) REFERENCES race(id)
);

CREATE INDEX IF NOT EXISTS race_id_idx               ON result (race_id);
CREATE INDEX IF NOT EXISTS race_id_horse_id_idx      ON result (race_id, horse_id);
CREATE INDEX IF NOT EXISTS race_id_jockey_id_idx     ON result (race_id, jockey_id);
CREATE INDEX IF NOT EXISTS race_id_trainer_id_idx    ON result (race_id, trainer_id);
CREATE INDEX IF NOT EXISTS race_id_owner_id_idx      ON result (race_id, owner_id);
CREATE INDEX IF NOT EXISTS horse_id_speed_index_idx ON result (horse_id, speed_index);
CREATE INDEX IF NOT EXISTS trainer_id_idx            ON result (trainer_id);
CREATE INDEX IF NOT EXISTS owner_id_idx              ON result (owner_id);
CREATE INDEX IF NOT EXISTS jockey_id_idx             ON result (jockey_id);

CREATE TABLE IF NOT EXISTS `payout` (
    race_id      INTEGER NOT NULL,
    ticket_type  TEXT    NOT NULL,
    draw         TEXT    NOT NULL,
    amount       REAL    NOT NULL,
    popularity   INTEGER NOT NULL,
    PRIMARY KEY (race_id, ticket_type, draw),
    FOREIGN KEY (race_id) REFERENCES race(id)
);

CREATE TABLE IF NOT EXISTS `horse` (
    id      TEXT    NOT NULL,
    name    TEXT    NOT NULL,
    born    INTEGER NOT NULL,
    sire_id TEXT,
    dam_id  TEXT,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS sire_id_idx ON horse (sire_id);
CREATE INDEX IF NOT EXISTS dam_id_idx ON horse (dam_id);