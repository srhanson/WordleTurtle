package app

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type DB interface {
	putResult(result Result) error
	getDailyResults(wordlenum int) ([]Result, error)
	getLargestWordle() (int, error)
}

type SQLiteDB struct {
	db *sql.DB
}

func NewSQLiteDB(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", path)
	return &SQLiteDB{db: db}, err
}

func (db *SQLiteDB) putResult(result Result) error {
	_, err := db.db.Exec("INSERT INTO results(wordlenum, userId, displayName, score, hardmode) VALUES( ?, ?, ?, ?, ? )", result.wordlenum, result.userId, result.displayName, result.score, result.hardmode)
	return err
}

func (db *SQLiteDB) getDailyResults(wordlenum int) ([]Result, error) {
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

func (db *SQLiteDB) getLargestWordle() (int, error) {
	row := db.db.QueryRow("SELECT MAX(wordlenum) FROM results")
	var max int
	err := row.Scan(&max)
	return max, err
}
