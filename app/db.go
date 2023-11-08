package app

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

func NewDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	return &DB{db: db}, err
}

func (db *DB) putResult(result Result) error {
	_, err := db.db.Exec("INSERT INTO results(wordlenum, userId, displayName, score, hardmode) VALUES( ?, ?, ?, ?, ? )", result.wordlenum, result.userId, result.displayName, result.score, result.hardmode)
	return err
}

func (db *DB) getDailyResults(wordlenum int) ([]Result, error) {
	rows, err := db.db.Query("SELECT wordlenum, userId, displayName, score, hardmode FROM results where wordlenum=?", wordlenum)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0)
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.wordlenum, &r.userId, &r.displayName, &r.score, &r.hardmode); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
