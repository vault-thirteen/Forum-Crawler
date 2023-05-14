package db

import (
	"os"
	"strings"
)

const (
	QueryCreateForumsTable = `CREATE TABLE IF NOT EXISTS Forums (
  ID INT UNSIGNED NOT NULL,
  Name VARCHAR(1024) NOT NULL,
  PRIMARY KEY (ID),
  UNIQUE INDEX ID_UNIQUE (ID ASC) VISIBLE
) 
ENGINE = InnoDB
DEFAULT CHARACTER SET = utf8;`

	QueryCreateTopicsTable = `CREATE TABLE IF NOT EXISTS Topics (
  ID INT UNSIGNED NOT NULL,
  Name VARCHAR(1024) NOT NULL,
  ForumId INT UNSIGNED NOT NULL,
  PRIMARY KEY (ID),
  UNIQUE INDEX ID_UNIQUE (ID ASC) VISIBLE,
  INDEX ForumId_Index (ForumId)
)
ENGINE = InnoDB
DEFAULT CHARACTER SET = utf8;`

	QueryUpsertForum = `INSERT INTO Forums (ID, Name) VALUES (?, ?) ON DUPLICATE KEY UPDATE ID=?, Name=?;`
	//QueryUpsertForum = `REPLACE INTO Forums (ID, Name) VALUES (?, ?);` // REPLACE is bugged in MySQL.

	QueryUpsertTopic = `INSERT INTO Topics (ID, Name, ForumId) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE ID=?, Name=?, ForumId=?;`
	//QueryUpsertTopic = `REPLACE INTO Topics (ID, Name, ForumId) VALUES (?, ?, ?);` // REPLACE is bugged in MySQL.

	QueryInsertNewTopic         = `INSERT IGNORE INTO Topics (ID, Name, ForumId) VALUES (?, ?, ?);`
	QueryInsertNewArchivedTopic = `INSERT IGNORE INTO TopicsArchived (ID, Name, ForumId) VALUES (?, ?, ?);`

	BulkThresholdCount = 10
)

const (
	PreparedStatementIdx_QueryUpsertForum            = 0
	PreparedStatementIdx_QueryUpsertTopic            = 1
	PreparedStatementIdx_QueryInsertNewTopic         = 2
	PreparedStatementIdx_QueryInsertNewArchivedTopic = 3
)

func escapeString(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `'`, `''`)
}

func saveQueryToFile(file string, query string) (err error) {
	return os.WriteFile(file, []byte(query), 0644)
}
