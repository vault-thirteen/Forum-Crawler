package db

import (
	"database/sql"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/vault-thirteen/Forum-Crawler/src/models"
	ae "github.com/vault-thirteen/auxie/errors"
)

type DB struct {
	conn               *sql.DB
	preparedStatements []*sql.Stmt
	tempFolder         string
}

func NewDB(settings *models.DatabaseSettings) (db *DB, err error) {
	db = &DB{
		tempFolder: settings.TemporaryFolder,
	}

	mc := mysql.Config{
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(settings.Host, strconv.FormatUint(uint64(settings.Port), 10)),
		DBName:               settings.Db,
		User:                 settings.User,
		Passwd:               settings.Password,
		AllowNativePasswords: true,
		CheckConnLiveness:    true,
		Params:               map[string]string{},
	}
	dsn := mc.FormatDSN()
	db.conn, err = sql.Open(settings.Driver, dsn)
	if err != nil {
		return nil, err
	}

	err = db.conn.Ping()
	if err != nil {
		return nil, err
	}

	err = db.InitTables()
	if err != nil {
		return nil, err
	}

	err = db.PrepareStatements()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() (err error) {
	err = db.CloseStatements()
	if err != nil {
		return err
	}

	return db.conn.Close()
}

func (db *DB) InitTables() (err error) {
	_, err = db.conn.Exec(QueryCreateForumsTable)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(QueryCreateTopicsTable)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) PrepareStatements() (err error) {
	db.preparedStatements = make([]*sql.Stmt, 0)

	var st *sql.Stmt
	{
		st, err = db.conn.Prepare(QueryUpsertForum) // 0.
		if err != nil {
			return err
		}
		db.preparedStatements = append(db.preparedStatements, st)
	}
	{
		st, err = db.conn.Prepare(QueryUpsertTopic) // 1.
		if err != nil {
			return err
		}
		db.preparedStatements = append(db.preparedStatements, st)
	}
	{
		st, err = db.conn.Prepare(QueryInsertNewTopic) // 2.
		if err != nil {
			return err
		}
		db.preparedStatements = append(db.preparedStatements, st)
	}
	{
		st, err = db.conn.Prepare(QueryInsertNewArchivedTopic) // 3.
		if err != nil {
			return err
		}
		db.preparedStatements = append(db.preparedStatements, st)
	}
	return nil
}

func (db *DB) CloseStatements() (err error) {
	for _, st := range db.preparedStatements {
		err = st.Close()
		if err != nil {
			return err
		}
	}

	db.preparedStatements = nil

	return nil
}

func (db *DB) SaveForum(forum *models.Forum) (err error) {
	var tx *sql.Tx
	tx, err = db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			derr := tx.Rollback()
			if derr != nil {
				err = ae.Combine(err, derr)
			}
		}
	}()

	st := tx.Stmt(db.preparedStatements[PreparedStatementIdx_QueryUpsertForum])
	defer func() {
		derr := st.Close()
		if derr != nil {
			err = ae.Combine(err, derr)
		}
	}()

	_, err = st.Exec(forum.ID, forum.Name,
		forum.ID, forum.Name)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) SaveTopic(topic *models.Topic) (err error) {
	var tx *sql.Tx
	tx, err = db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			derr := tx.Rollback()
			if derr != nil {
				err = ae.Combine(err, derr)
			}
		}
	}()

	st := tx.Stmt(db.preparedStatements[PreparedStatementIdx_QueryUpsertTopic])
	defer func() {
		derr := st.Close()
		if derr != nil {
			err = ae.Combine(err, derr)
		}
	}()

	_, err = st.Exec(topic.Id, topic.Name, topic.ForumId,
		topic.Id, topic.Name, topic.ForumId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) SaveNewTopic(topic *models.Topic, isArchived bool) (err error) {
	var tx *sql.Tx
	tx, err = db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			derr := tx.Rollback()
			if derr != nil {
				err = ae.Combine(err, derr)
			}
		}
	}()

	var st *sql.Stmt
	if isArchived {
		st = tx.Stmt(db.preparedStatements[PreparedStatementIdx_QueryInsertNewArchivedTopic])
	} else {
		st = tx.Stmt(db.preparedStatements[PreparedStatementIdx_QueryInsertNewTopic])
	}
	defer func() {
		derr := st.Close()
		if derr != nil {
			err = ae.Combine(err, derr)
		}
	}()

	_, err = st.Exec(topic.Id, topic.Name, topic.ForumId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) SaveTopics(forumId uint, topics map[uint]*models.Topic) (err error) {
	var topicsList = make([]*models.Topic, 0, len(topics))
	for _, topic := range topics {
		topicsList = append(topicsList, topic)
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`INSERT INTO Topics (ID, Name, ForumId) VALUES `)

	iMax := len(topicsList) - 2
	for i := 0; i <= iMax; i++ {
		queryBuilder.WriteString("(" +
			strconv.FormatUint(uint64(topicsList[i].Id), 10) + "," + // ID.
			`'` + escapeString(topicsList[i].Name) + `',` + // Name.
			strconv.FormatUint(uint64(topicsList[i].ForumId), 10) + "),\r\n") // ForumId.
	}
	iMax++

	queryBuilder.WriteString("(" +
		strconv.FormatUint(uint64(topicsList[iMax].Id), 10) + "," + // ID.
		`'` + escapeString(topicsList[iMax].Name) + `',` + // Name.
		strconv.FormatUint(uint64(topicsList[iMax].ForumId), 10) + ");") // ForumId.

	var tx *sql.Tx
	tx, err = db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			derr := tx.Rollback()
			if derr != nil {
				err = ae.Combine(err, derr)
			}
		}
	}()

	query := queryBuilder.String()

	queryFilePath := filepath.Join(db.tempFolder, fmt.Sprintf("forum_%v.sql", forumId))
	err = saveQueryToFile(queryFilePath, query)
	if err != nil {
		return err
	}

	_, err = tx.Exec(query)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
